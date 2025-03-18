import sqlite3
import pymysql
import subprocess
import re
from collections import defaultdict
from datetime import datetime

# 🔹 Zabbix Database Configuration
zabbix_db_host = "localhost"
zabbix_db_user = "zabbix"
zabbix_db_password = "zabbixpass"
zabbix_db_name = "zabbix"

# 🔹 SQLite Database Configuration
sqlite_db = "network_inventory.db"

# 🔹 Group ID to Query in Zabbix
zabbix_group_id = 28

# 📌 1️⃣ Fetch SNMP Hosts from Zabbix (Including Vendor)
def get_zabbix_hosts():
    conn = pymysql.connect(host=zabbix_db_host, user=zabbix_db_user, password=zabbix_db_password, database=zabbix_db_name)
    cursor = conn.cursor()

    query = """
    SELECT h.host, hi.ip, hi.community, ht.templateid
    FROM hosts h
    JOIN hosts_groups hg ON h.hostid = hg.hostid
    JOIN interface hi ON h.hostid = hi.hostid
    JOIN hosts_templates ht ON h.hostid = ht.hostid
    WHERE hg.groupid = %s AND hi.type = 2
    """
    
    cursor.execute(query, (zabbix_group_id,))
    results = cursor.fetchall()
    conn.close()

    host_data = []
    for row in results:
        hostname, ip, community, templateid = row
        if templateid == 10250:
            vendor = "ProCurve"
        elif templateid == 10251:
            vendor = "Cisco"
        elif templateid == 10252:
            vendor = "Aruba"
        else:
            vendor = "Unknown"

        host_data.append({"hostname": hostname, "ip": ip, "community": community, "vendor": vendor})

    return host_data

# 📌 2️⃣ Get MAC Address → Port Mapping via SNMP
def get_mac_port_data(switch_ip, snmp_community, vendor):
    mac_table_oid = "1.3.6.1.2.1.17.4.3.1.2"
    interface_oid = "1.3.6.1.2.1.2.2.1.2"

    vlan_oid = {
        "Cisco": "1.3.6.1.4.1.9.9.68.1.2.2.1.2",
        "Aruba": "1.3.6.1.2.1.17.7.1.4.5.1.1",
        "ProCurve": "1.3.6.1.2.1.17.7.1.4.5.1.1"
    }.get(vendor, None)

    if vlan_oid is None:
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

# 📌 3️⃣ Store Data in SQLite3 (with Created/Updated Timestamps)
def update_sqlite_database(switch_name, switch_ip, vendor, mac_data):
    conn = sqlite3.connect(sqlite_db)
    cursor = conn.cursor()

    # Create Table with Timestamp Fields
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

# 📌 4️⃣ Execute the Workflow
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
