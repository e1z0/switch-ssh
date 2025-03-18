import subprocess
import re

# Run SNMP command
output = subprocess.run(
    ["snmpwalk", "-v2c", "-c", "public", "192.168.1.1", "1.3.6.1.2.1.17.4.3.1.2"],
    capture_output=True, text=True
)

# Parse output
for line in output.stdout.split("\n"):
    match = re.search(r'(\d+\.\d+\.\d+\.\d+\.\d+\.\d+) = INTEGER: (\d+)', line)
    if match:
        mac_raw = match.group(1).split('.')
        mac_hex = ":".join(f"{int(x):02X}" for x in mac_raw)
        port = match.group(2)
        print(f"MAC Address: {mac_hex} → Port: {port}")
