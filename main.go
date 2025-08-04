package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/kolo/xmlrpc"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Hostname string
	Port     string
	User     string
	Password string
}

type AddPackagesConfig struct {
	Name           string   `yaml:"name"`
	Version        string   `yaml:"version"`
	Release        string   `yaml:"release"`
	SourceChannel  string   `yaml:"source_channel"`
	TargetChannels []string `yaml:"target_channels"`
}

type Package struct {
	Name      string `xmlrpc:"name"`
	Version   string `xmlrpc:"version"`
	Release   string `xmlrpc:"release"`
	Epoch     string `xmlrpc:"epoch"`
	ID        int    `xmlrpc:"id"`
	ArchLabel string `xmlrpc:"arch_label"`
}

type Pkg_ProvidingChannels struct {
	Label       string `xmlrpc:"label"`
	ParentLabel string `xmlrpc:"parent_label"`
	Name        string `xmlrpc:"name"`
}

func NewConfigFromEnv() *Config {
	return &Config{
		Hostname: getenvOrDefault("SUSE_MANAGER_HOSTNAME", "mysuma1.susedemo.de"),
		Port:     getenvOrDefault("SUSE_MANAGER_PORT", "443"),
		User:     getenvOrDefault("SUSE_MANAGER_USER", "apiuser"),
		Password: getenvOrDefault("SUSE_MANAGER_PASSWORD", "suselinux"),
	}
}

func getenvOrDefault(key, def string) string {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	return val
}

func loginToSMLM(config *Config, user, password string) (*xmlrpc.Client, string, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client, err := xmlrpc.NewClient(fmt.Sprintf("https://%s:%s/rpc/api",
		config.Hostname, config.Port), transport)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create XML-RPC client: %v", err)
	}

	var sessionKey string
	err = client.Call("auth.login", []interface{}{user, password}, &sessionKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to login: %v", err)
	}
	return client, sessionKey, nil
}

func logoutFromSMLM(client *xmlrpc.Client, sessionKey string) error {
	var result int
	err := client.Call("auth.logout", []interface{}{sessionKey}, &result)
	if err != nil {
		return fmt.Errorf("failed to logout: %v", err)
	}

	if result == 1 {
		fmt.Println("\033[32mSuccessfully logged out from SUSE Manager\033[0m")
	} else {
		return fmt.Errorf("\033[31mfailed to logout from SUSE Manager\033[0m")
	}
	return nil
}

func addPackageToChannel(client *xmlrpc.Client, sessionKey string, pkgIDs []int, channelLabel string) error {

	var result int
	err := client.Call("channel.software.addPackages", []interface{}{sessionKey, channelLabel, pkgIDs}, &result)
	if err != nil {
		return fmt.Errorf("failed to add packages to channel: %v", err)
	}

	if result == 1 {
		fmt.Printf("\033[32mSuccessfully added packages to channel\033[0m %s\n", channelLabel)
	} else {
		return fmt.Errorf("\033[31mfailed to add packages to channel\033[0m %s", channelLabel)
	}
	return nil
}

func listPackagesInChannel(client *xmlrpc.Client, sessionKey string, channelLabel string) ([]Package, error) {
	var packages []Package
	//err := client.Call("channel.software.listLatestPackages", []interface{}{sessionKey, channelLabel}, &packages)
	err := client.Call("channel.software.listAllPackages", []interface{}{sessionKey, channelLabel}, &packages)
	if err != nil {
		return nil, fmt.Errorf("failed to list packages in channel %s: %v", channelLabel, err)
	}

	if len(packages) == 0 {
		fmt.Printf("\033[33mNo packages found in channel\033[0m %s\n", channelLabel)
	}
	return packages, nil
}

func listProvidingChannels(client *xmlrpc.Client, sessionKey string, packageID int) ([]Pkg_ProvidingChannels, error) {
	var pkg_channels []Pkg_ProvidingChannels
	err := client.Call("packages.listProvidingChannels", []interface{}{sessionKey, packageID}, &pkg_channels)
	if err != nil {
		return nil, fmt.Errorf("failed to list providing channels for package %d: %v", packageID, err)
	}
	return pkg_channels, nil
}

func handleAddPackages(config *Config, yamlFile string) {
	file, err := os.Open(yamlFile)
	if err != nil {
		log.Fatalf("failed to open yaml file: %v", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	var addPackagesConfig []AddPackagesConfig
	if err := decoder.Decode(&addPackagesConfig); err != nil {
		log.Fatalf("failed to decode yaml file: %v", err)
	}

	//log.Printf("Loaded configuration from %s: %+v\n", yamlFile, addPackagesConfig)
	if config.Password == "" {
		log.Fatal("SUSE_MANAGER_PASSWORD environment variable is required")
	}

	fmt.Printf("Connecting to SUSE Manager at %s:%s with user %s\n",
		config.Hostname, config.Port, config.User)

	client, sessionKey, err := loginToSMLM(config, config.User, config.Password)
	if err != nil {
		log.Fatalf("\033[31mfailed to login to SUSE Manager\033[0m: %v", err)
	}
	defer logoutFromSMLM(client, sessionKey)

	for _, cfg := range addPackagesConfig {
		fmt.Printf("Processing package: %s\n", cfg.Name)
		if cfg.SourceChannel == "" || len(cfg.TargetChannels) == 0 {
			log.Printf("\033[31mSkipping package %s: source channel or target channels not specified\033[0m", cfg.Name)
			continue
		}
		sourcePackages, err := listPackagesInChannel(client, sessionKey, cfg.SourceChannel)
		if err != nil {
			log.Printf("\033[31mFailed to list packages in source channel %s: %v\033[0m", cfg.SourceChannel, err)
			continue
		}

		var pkgIDs []int
		for _, pkg := range sourcePackages {
			if pkg.Name == cfg.Name && pkg.Version == cfg.Version && pkg.Release == cfg.Release {
				fmt.Printf("\n\033[32mFound package:\033[0m %s (ID: %d) in channel %s\n", cfg.Name, pkg.ID, cfg.SourceChannel)
				found_pkgs, err := listProvidingChannels(client, sessionKey, pkg.ID)
				if err != nil {
					log.Printf("\033[31mFailed to list providing channels for package %d: %v\033[0m", pkg.ID, err)
					continue
				}
				if len(found_pkgs) == 0 {
					log.Printf("\033[33mNo providing channels found for package %s (ID: %d)\033[0m", pkg.Name, pkg.ID)
				} else {
					for _, targetChannel := range cfg.TargetChannels {
						pkg_already_in_target := false
						for _, channel := range found_pkgs {
							//fmt.Printf("\033[34mPackage %s (ID: %d) is provided by channel: %s\033[0m\n", pkg.Name, pkg.ID, channel.Label)
							if channel.Label == targetChannel {
								pkg_already_in_target = true
								fmt.Printf("\033[32mPackage %s (ID: %d) is already in target channel %s\033[0m\n", pkg.Name, pkg.ID, targetChannel)
							}
						}

						if !pkg_already_in_target {
							pkgIDs = append(pkgIDs, pkg.ID)
							//fmt.Printf("\033[32mAdding package ID %d to the list for target channels\033[0m\n", pkg.ID)
							if len(pkgIDs) > 0 {
								fmt.Printf("\033[34mAdding package:\033[0m %s (IDs: %v) to target channel %s\n", cfg.Name, pkgIDs, targetChannel)
								if err := addPackageToChannel(client, sessionKey, pkgIDs, targetChannel); err != nil {
									log.Printf("\033[31mFailed to add package:\033[0m %s to channel %s: %v", cfg.Name, targetChannel, err)
								}
							}
						} else {
							log.Printf("\033[33mPackage %s (ID: %d) is already in target channel %s, skipping...\033[0m", pkg.Name, pkg.ID, targetChannel)
						}
					}
				}

			}
		}

		/* if len(pkgIDs) == 0 {
			log.Printf("\033[31mPackage not found:\033[0m %s in channel %s with version %s and release %s", cfg.Name, cfg.SourceChannel, cfg.Version, cfg.Release)
			continue
		} */
	}
}

func handleListPackages(config *Config, channelLabel string) {
	if config.Password == "" {
		log.Fatal("SUSE_MANAGER_PASSWORD environment variable is required")
	}

	fmt.Printf("Connecting to SUSE Manager at %s:%s with user %s\n",
		config.Hostname, config.Port, config.User)

	client, sessionKey, err := loginToSMLM(config, config.User, config.Password)
	if err != nil {
		log.Fatalf("\033[31mfailed to login to SUSE Manager\033[0m: %v", err)
	}
	defer logoutFromSMLM(client, sessionKey)

	packages, err := listPackagesInChannel(client, sessionKey, channelLabel)
	if err != nil {
		log.Fatalf("\033[31mfailed to list packages in channel\033[0m %s: %v", channelLabel, err)
	}

	for _, pkg := range packages {
		fmt.Printf("Package: %s, Version: %s, Release: %s, Arch: %s, ID: %d\n", pkg.Name, pkg.Version, pkg.Release, pkg.ArchLabel, pkg.ID)
	}

	fmt.Printf("\033[32mFound %d packages in channel\033[0m %s\n", len(packages), channelLabel)
}

func main() {
	helpCmd := flag.NewFlagSet("help", flag.ExitOnError)
	if len(os.Args) < 2 {
		helpCmd.Parse(os.Args[1:])
	}
	if len(os.Args) == 1 || os.Args[1] == "help" {
		fmt.Println("Usage:")
		fmt.Println("  smlm_tool <command> [options]")
		fmt.Println("Subcommand:")
		fmt.Println("  add_packages --config <pkg_list.yaml> - Add packages to channels based on the configuration in the YAML file.")
		fmt.Println("Flags:")
		fmt.Println("  --config <path> - Path to the configuration file (not supported in this version).")
		fmt.Println()
		fmt.Println("Subcommand:")
		fmt.Println("  list_packages --channel <channel_label> - List packages in the specified channel.")
		fmt.Println("Flags:")
		fmt.Println("  --channel <label> - Channel label to list packages from (not supported in this version).")
		fmt.Println()
		fmt.Println("Environment Variables:")
		fmt.Println("  SUSE_MANAGER_HOSTNAME - Hostname of the SUSE Manager server.")
		fmt.Println("  SUSE_MANAGER_PORT - Port of the SUSE Manager server (default is 443).")
		fmt.Println("  SUSE_MANAGER_USER - Username for SUSE Manager.")
		fmt.Println("  SUSE_MANAGER_PASSWORD - Password for SUSE Manager.")
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  smlm_tool add_packages --config pkg_list.yaml - Add packages defined in pkg_list.yaml to the specified channels.")
		fmt.Println("  smlm_tool list_packages --channel <channel_label> - List all packages in the specified channel.")
		return
	}

	addPackagesCmd := flag.NewFlagSet("add_packages", flag.ExitOnError)
	addPackagesCmd_config := addPackagesCmd.String("config", "", "Path to configuration file")

	listPackagesCmd := flag.NewFlagSet("list_packages", flag.ExitOnError)
	listPackagesCmd_channel := listPackagesCmd.String("channel", "", "Path to configuration file")
	if len(os.Args) < 2 {
		log.Fatal("expected 'add_packages' or 'list_packages' subcommand")
	}

	config := NewConfigFromEnv()
	if config.Hostname == "" || config.Port == "" || config.User == "" {
		log.Fatal("SUSE_MANAGER_HOSTNAME, SUSE_MANAGER_PORT, and SUSE_MANAGER_USER environment variables are required")
	}

	switch os.Args[1] {
	case "add_packages":
		addPackagesCmd.Parse(os.Args[2:])
		log.Default().Println("addPackagesCmd_config", *addPackagesCmd_config)
		if *addPackagesCmd_config == "" {
			log.Fatal("The --config flag is not used.")
		}
		/* if len(addPackagesCmd.Args()) < 1 {
			log.Fatal("Usage: main add_packages --config <pkg_list.yaml>")
		} */
		yamlFile := *addPackagesCmd_config
		if yamlFile == "" {
			log.Fatal("Usage: smlm_tool add_packages --config <pkg_list.yaml>")
		}
		if _, err := os.Stat(yamlFile); os.IsNotExist(err) {
			log.Fatalf("YAML file does not exist: %s", yamlFile)
		}
		if _, err := os.Stat(yamlFile); err != nil {
			if os.IsPermission(err) {
				log.Fatalf("Permission denied accessing YAML file: %s. Please check file permissions.", yamlFile)
			}
			log.Fatalf("Error accessing YAML file: %v", err)
		}
		if yamlFile == "" {
			log.Fatal("Usage: smlm_tool add_packages --config <pkg_list.yaml>")
		}
		handleAddPackages(NewConfigFromEnv(), yamlFile)

	case "list_packages":
		listPackagesCmd.Parse(os.Args[2:])
		if *listPackagesCmd_channel == "" {
			log.Fatal("The --channel flag is not supported in this version. Please use environment variables instead.")
		}
		/* if len(listPackagesCmd.Args()) < 1 {
			log.Fatal("Usage: smlm_tool list_packages --channel <channel_label>")
		} */
		channelLabel := *listPackagesCmd_channel
		if channelLabel == "" {
			log.Fatal("Usage: smlm_tool list_packages --channel <channel_label>")
		}
		handleListPackages(NewConfigFromEnv(), channelLabel)
	default:
		log.Fatalf("unknown subcommand: %s", os.Args[1])
	}

}
