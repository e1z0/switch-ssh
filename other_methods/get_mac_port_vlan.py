import subprocess
import re
from collections import defaultdict

# SNMP settings
switch_ip = "192.168.10.2"
snmp_community = "LAB08PUB"

# 1️⃣ Get MAC Address Table (MAC → ifIndex)
mac_table = subprocess.run(
    ["snmpwalk", "-v2c", "-c", snmp_community, switch_ip, "1.3.6.1.2.1.17.4.3.1.2"],
    capture_output=True, text=True
).stdout

# 2️⃣ Get Interface Index to Port Name Mapping (ifIndex → Port Name)
interface_table = subprocess.run(
    ["snmpwalk", "-v2c", "-c", snmp_community, switch_ip, "1.3.6.1.2.1.2.2.1.2"],
    capture_output=True, text=True
).stdout

# 3️⃣ Get VLAN Table (ifIndex → VLAN)
vlan_table = subprocess.run(
    ["snmpwalk", "-v2c", "-c", snmp_community, switch_ip, "1.3.6.1.4.1.9.9.68.1.2.2.1.2"],
    capture_output=True, text=True
).stdout

# Convert ifIndex → Port Name
ifIndex_to_port = {}
for line in interface_table.split("\n"):
    match = re.search(r'(\d+) = STRING: "([^"]+)"', line)
    if match:
        ifIndex_to_port[match.group(1)] = match.group(2)

# Convert ifIndex → VLAN
ifIndex_to_vlan = {}
for line in vlan_table.split("\n"):
    match = re.search(r'(\d+) = INTEGER: (\d+)', line)
    if match:
        ifIndex_to_vlan[match.group(1)] = match.group(2)

# Store MAC addresses mapped to ports & VLANs
mac_to_info = defaultdict(list)

# Process MAC Table
for line in mac_table.split("\n"):
    match = re.search(r'(\d+\.\d+\.\d+\.\d+\.\d+\.\d+) = INTEGER: (\d+)', line)
    if match:
        mac_raw = match.group(1).split('.')
        mac_hex = ":".join(f"{int(x):02X}" for x in mac_raw)
        ifIndex = match.group(2)
        port_name = ifIndex_to_port.get(ifIndex, "Unknown Port")
        vlan_id = ifIndex_to_vlan.get(ifIndex, "Unknown VLAN")

        mac_to_info[mac_hex].append((port_name, vlan_id))

# Print Results
for mac, info in mac_to_info.items():
    port_list = ", ".join([f"{port} (VLAN {vlan})" for port, vlan in info])
    print(f"MAC Address: {mac} → {port_list}")
