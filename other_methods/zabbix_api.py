import requests
import json
import sqlite3
import subprocess
import re
from collections import defaultdict
from datetime import datetime

# 🔹 Zabbix API Configuration
zabbix_url = "http://your-zabbix-server/api_jsonrpc.php"
zabbix_user = "Admin"
zabbix_password = "zabbix"
zabbix_group_id = 28

# 🔹 SQLite Database Configuration
sqlite_db = "network_inventory.db"

# 📌 1️⃣ Authenticate to Zabbix API
def zabbix_api_request(method, params):
    headers = {"Content-Type": "application/json"}
    auth_payload = {
        "jsonrpc": "2.0",
        "method": "user.login",
        "params": {"user": zabbix_user, "password": zabbix_password},
        "id": 1
    }
    
    auth_response = requests.post(zabbix_url, data=json.dumps(auth_payload), headers=headers)
    auth_token = auth_response.json().get("result")

    if not auth_token:
        raise Exception("❌ Zabbix Authentication Failed!")

    payload = {
        "jsonrpc": "2.0",
        "method": method,
        "params": params,
        "auth": auth_token,
        "id": 2
    }

    response = requests.post(zabbix_url, data=json.dumps(payload), headers=headers)
    return response.json().get("result")

# 📌 2️⃣ Retrieve Hosts + SNMP Details via API
def get_zabbix_hosts():
    hosts = zabbix_api_request("host.get", {
        "groupids": zabbix_group_id,
        "output": ["host"],
        "selectInterfaces": ["ip", "details"],
        "selectParentTemplates": ["templateid"]
    })

    host_data = []
    for host in hosts:
        hostname = host["host"]
        interfaces = host.get("interfaces", [])

        if not interfaces:
            continue

        # Find SNMP Interface
        snmp_interface = next((iface for iface in interfaces if iface["type"] == "2"), None)
        if not snmp_interface:
            continue

        ip = snmp_interface["ip"]
        snmp_community = snmp_interface["details"].get("community", "public")

        # Detect Vendor
        template_ids = [t["templateid"] for t in host.get("parentTemplates", [])]
        vendor = "Unknown"
        if "10250" in template_ids:
            vendor = "ProCurve"
        elif "10251" in template_ids:
            vendor = "Cisco"
        elif "10252" in template_ids:
            vendor = "Aruba"

        host_data.append({"hostname": hostname, "ip": ip, "community": snmp_community, "vendor": vendor})

    return host_data

# 📌 3️⃣ Get MAC Addresses and Ports via SNMP
def get_mac_port_data(switch_ip, snmp_community, vendor):
    mac_table_oid = "1.3.6.1.2.1.17.4.3.1.2"
    interface_oid = "1.3.6.1.2.1.2.2.1.2"

    vlan_oid = {
        "Cisco": "1.3.6.1.4.1.9.9.68.1.2.2.1.2",
        "Aruba": "1.3.6.1.2.1.17.7.1.4.5.1.1",
        "ProCurve": "1.3.6.1.2.1.17.7.1.4.5.1.1"
    }.get(vendor, None)

    if not vlan_oid:
        print(f"❌ Unknown Vendor for {switch_ip}. Skipping...")
        return []

    mac_table = subprocess.run(["snmpwalk", "-v2c", "-c", snmp_community, switch_ip, mac_table_oid], capture_output=True, text=True).stdout
    interface_table = subprocess.run(["snmpwalk", "-v2c", "-c", snmp_community, switch_ip, interface_oid], capture_output=True, text=True).stdout
    vlan_table = subprocess.run(["snmpwalk", "-v2c", "-c", snmp_community, switch_ip, vlan_oid], capture_output=True, text=True).stdout

    ifIndex_to_port = {}
    for line in interface_table.split("\n"):
        match = re.search(r'(\d+) = STRING: "([^"]+)"', line)
        if match:
            ifIndex_to_port[match.group(1)] = match.group(2)

    ifIndex_to_vlan = {}
    for line in vlan_table.split("\n"):
        match = re.search(r'(\d+) = INTEGER: (\d+)', line)
        if match:
            ifIndex_to_vlan[match.group(1)] = match.group(2)

    mac_port_data = []
    for line in mac_table.split("\n"):
        match = re.search(r'(\d+\.\d+\.\d+\.\d+\.\d+\.\d+) = INTEGER: (\d+)', line)
        if match:
            mac_raw = match.group(1).split('.')
            mac_hex = ":".join(f"{int(x):02X}" for x in mac_raw)
            ifIndex = match.group(2)
            port_name = ifIndex_to_port.get(ifIndex, "Unknown Port")
            vlan_id = ifIndex_to_vlan.get(ifIndex, "Unknown VLAN")

            mac_port_data.append((mac_hex, port_name, vlan_id))

    return mac_port_data

# 📌 4️⃣ Store Data in SQLite3
def update_sqlite_database(switch_name, switch_ip, vendor, mac_data):
    conn = sqlite3.connect(sqlite_db)
    cursor = conn.cursor()

    cursor.execute("""
    CREATE TABLE IF NOT EXISTS network_inventory (
        switch_name TEXT,
        switch_ip TEXT,
        vendor TEXT,
        mac_address TEXT,
        port_name TEXT,
        vlan TEXT,
        created_at TEXT DEFAULT CURRENT_TIMESTAMP,
        updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(switch_name, mac_address, port_name, vlan)
    )
    """)

    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")

    for mac, port, vlan in mac_data:
        cursor.execute("""
        INSERT INTO network_inventory (switch_name, switch_ip, vendor, mac_address, port_name, vlan, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(switch_name, mac_address, port_name, vlan)
        DO UPDATE SET updated_at = excluded.updated_at
        """, (switch_name, switch_ip, vendor, mac, port, vlan, now, now))

    conn.commit()
    conn.close()

# 📌 5️⃣ Run the Workflow
def main():
    hosts = get_zabbix_hosts()

    for host in hosts:
        print(f"🔄 Querying {host['hostname']} ({host['ip']}) - {host['vendor']}...")
        mac_port_data = get_mac_port_data(host['ip'], host['community'], host['vendor'])

        if mac_port_data:
            update_sqlite_database(host['hostname'], host['ip'], host['vendor'], mac_port_data)
            print(f"✅ Data for {host['hostname']} updated.")
        else:
            print(f"⚠️ No MAC data found for {host['hostname']}.")

if __name__ == "__main__":
    main()
