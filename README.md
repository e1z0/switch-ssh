# Network Device OS Detection and Configuration Tool

This Golang-based application is designed to connect to network switches and routers, intelligently detect their operating systems, and execute commands for configuration management. The tool automates network operations by identifying vendor-specific command structures and adapting accordingly.

## Current Features:

✔ OS Detection & Signature Matching – Automatically identifies the switch/router operating system based on predefined signatures, ensuring compatibility with various vendors.

✔ Paging Control – Disables pagination effectively, adapting to different CLI environments.

✔ Configuration Backup – Executes show running-config to retrieve and store device configurations.

✔ Multi-Vendor Support – Successfully tested on:

* Cisco SBOS, Cisco IOS, Cisco IOS XE, Cisco NX-OS
* ArubaOS, ArubaOS-CX
* FortiOS

## Planned Features:

🚀 VLAN and SNMP Assignment – Enable seamless VLAN management and SNMP configuration.

🚀 API & Protocol Expansion – Introduce structured APIs for deeper interaction with network devices.

🚀 Full Network Automation – Support additional commands and programmable interactions with switches and routers.

This tool is built to unify and streamline network management, allowing administrators to execute standardized tasks across different vendors while handling vendor-specific command differences in the background.
