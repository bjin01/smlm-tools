# SMLM Tools
SUSE Multi-Linux Manager (SMLM) tools for managing packages and channels.
This tool is still under development and may not support all features yet.

At the moment, it supports adding packages to channels and listing packages in a specific channel.

__Motivation__:
Although SMLM provides content-lifecycle-management to clone channels and filter packages sometimes it is necessary to add single package to channels without the need to build and promote a complete channel. Especially re-build and promote many channels in large environments can take a lot of time and resources.
This tool allows you to do add packages by specifying the packages in a YAML configuration file.

# Tested with:
- SMLM 5.1
- OS: SLE-Micro 6.1

# Features
- Add packages to channels based on a YAML configuration file.
- List packages in a specific channel. Use this feature to find details about packages in a channel, such as version, release, architecture, but also verify if a package is already in the channel.
- if a package is already in the target channel, it will not be added again.
- If a package is not found in the source channel, it will be logged, and the process will continue with the next package.
- same package can be added to multiple target channels.
- same package with different versions can be added to the same target channel, a new package definition in yaml is required for each version.

# Installation
To install the tool, clone the repository and run:
```bash
go build -o smlm_tool main.go
```
# Prerequisites
- Go 1.24 or later
- Required Go packages:
  - `github.com/spf13/cobra`
  - `gopkg.in/yaml.v3`  

# Usage
Run the tool with the following command:
```bash
smlm_tool <command> [options]
```
# Sub-Commands
- `add_packages --config <pkg_list.yaml>`: Add packages to channels based on the configuration in the YAML file.
- `list_packages --channel <channel_label>`: List packages in a specific channel.

# Example
To add packages from a YAML configuration file:
```bash
smlm_tool add_packages --config pkg_list.yaml
```
To list packages in a specific channel:
```bash
smlm_tool list_packages --channel <channel_label>
```
# Run smlm-tool using container

You can run the tool using a containerized environment with Podman or Docker. The container image is built from the source code and includes all necessary dependencies.

# Build the container image
```bash
git clone https://github.com/bjin01/smlm-tools.git
cd smlm-tools
podman build -t smlm-tools .
```

The environment_file should contain the necessary environment variables to connect to your SUSE Manager instance, such as:
```bash
SUSE_MANAGER_URL=https://mysuma1.susedemo.de:443
SUSE_MANAGER_PORT=443
SUSE_MANAGER_USER=apiuser
SUSE_MANAGER_PASSWORD=apiuserpassword
```
```bash
podman run -it \
--env-file "$PWD"/environment_file \
-v "$PWD"/pkg_list.yaml:/pkg_list.yaml:Z \
smlm-tools /usr/local/bin/smlm_tool \
add_packages --config /pkg_list.yaml
```

# Configuration
The configuration for adding packages is specified in a YAML file. The file should contain the following structure:
```yaml
- <package_name>
  version: <package_version>
  release: <package_release>
  source_channel: <source_channel_label>
  target_channels: 
    - <target_channel_label>
    - <another_target_channel_label>

- <another_package_name>
  version: <another_package_version>
  release: <another_package_release>
  source_channel: <another_source_channel_label>
  target_channels: 
    - <another_target_channel_label>
    - <yet_another_target_channel_label>
```
# Output example:
```bash
smlm_tool add_packages --config=pkg_list.yaml
2025/08/03 14:50:20 addPackagesCmd_config pkg_list.yaml
Connecting to SUSE Manager at mysuma1.susedemo.de:443 with user apiuser
Processing package: asdfasdf
2025/08/03 14:50:21 Package not found: asdfasdf in channel sle-manager-tools15-updates-x86_64-sap-sp6 with version 3006.0 and release 150000.3.78.1
Processing package: venv-salt-minion
Found package: venv-salt-minion (ID: 34634) in channel sle-manager-tools15-updates-x86_64-sap-sp6
Adding package: venv-salt-minion (IDs: [34634]) to target channel sles15sp6-test-sle-manager-tools15-updates-x86_64-sap-sp6
Successfully added packages to channel sles15sp6-test-sle-manager-tools15-updates-x86_64-sap-sp6
Successfully logged out from SUSE Manager
```
```bash