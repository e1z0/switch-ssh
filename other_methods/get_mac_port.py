import subprocess
import re

# SNMP settings
switch_ip = "192.168.1.1"
snmp_community = "public"

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

# Convert ifIndex → Port Name
ifIndex_to_port = {}
for line in interface_table.split("\n"):
    match = re.search(r'(\d+) = STRING: "([^"]+)"', line)
    if match:
        ifIndex_to_port[match.group(1)] = match.group(2)

# Find MAC Addresses and their Ports
for line in mac_table.split("\n"):
    match = re.search(r'(\d+\.\d+\.\d+\.\d+\.\d+\.\d+) = INTEGER: (\d+)', line)
    if match:
        mac_raw = match.group(1).split('.')
        mac_hex = ":".join(f"{int(x):02X}" for x in mac_raw)
        ifIndex = match.group(2)
        port_name = ifIndex_to_port.get(ifIndex, "Unknown Port")

        print(f"MAC Address: {mac_hex} → Port: {port_name}")
