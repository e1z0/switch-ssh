package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
        "errors"
)

type OS struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Models      []string `json:"models"`
	Versions    []string `json:"versions"`
        Pager       string   `json:"pager"`
        MacAddrComm string   `json:"mac-addr-list"`
}

var IsLogDebug = true
var ver = "0.1"
var osData []OS

func loadOSData() error {
	data, err := os.ReadFile("devices.json")
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &osData)
	if err != nil {
		return err
	}

	return nil
}

func ReturnOsInfo(input string) (OS,error) {
   for _, osEntry := range osData {
       if osEntry.Name == input {
          return osEntry,nil
       }
   }
   return OS{},errors.New("no os of that name found")
}

// finds OS by model or version
func findOSByModelOrVersion(input string) *OS {
	for _, osEntry := range osData {
		// Check models
		for _, modelRegex := range osEntry.Models {
			matched, _ := regexp.MatchString(modelRegex, input)
			if matched {
				return &osEntry
			}
		}

		// Check versions
		for _, versionRegex := range osEntry.Versions {
			matched, _ := regexp.MatchString(versionRegex, input)
			if matched {
				return &osEntry
			}
		}
	}
	return nil
}

// Function to verify both model and version at the same time
func verifyModelAndVersion(modelInput, versionInput string) *OS {
	for _, osEntry := range osData {
		// Double-check both model and version using regex
		modelMatched := false
		versionMatched := false

		// Check models
		for _, modelRegex := range osEntry.Models {
			matched, _ := regexp.MatchString(modelRegex, modelInput)
			if matched {
				modelMatched = true
				break
			}
		}

		// Check versions
		for _, versionRegex := range osEntry.Versions {
			matched, _ := regexp.MatchString(versionRegex, versionInput)
			if matched {
				versionMatched = true
				break
			}
		}

		// Return the OS if both match
		if modelMatched && versionMatched {
			return &osEntry
		}
	}

	// Return nil if no match found
	return nil
}

func SaveFile(filename string, content string) error {
	file, err := os.Create(filename)
	if err != nil {
		fmt.Printf("Error creating file: %s\n", err)
		return err
	}
	defer file.Close()
	_, err = file.WriteString(content)
	if err != nil {
		fmt.Printf("Error writing to file: %s\n", err)
		return err
	}
	fmt.Println("Data written to file successfully.")
	return nil
}

func AppendFile(filename string, text string) error {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err = f.WriteString(text); err != nil {
		return err
	}
	return nil
}

func main() {
	mode := flag.String("mode", "detect", "The mode to run the application (e.g., detect, run, testmodel, mac")
	host := flag.String("host", "", "Hostname to connect to")
	port := flag.Int("port", 22, "A ssh port number")
	user := flag.String("user", "", "Username")
	pass := flag.String("pass", "", "Password")
	model := flag.String("model", "", "Device Model or version")
	version := flag.String("version", "", "Device version")
	debug := flag.Bool("debug", false, "Aggresive debugging on")
	mass := flag.Bool("mass", false, "Mass do")
	dump := flag.String("dump", "", "Dump command, example.: dump running-config")
	save := flag.String("save", "", "show command, for example running-config")
	flag.Parse()

	fmt.Printf("VER: %s\n", ver)

	IsLogDebug = *debug

	if *mode == "testmodel" && *model != "" {
		err := loadOSData()
		if err != nil {
			fmt.Printf("Error loading OS data: %v\n", err)
			return
		}
		if *version != "" {
			result := verifyModelAndVersion(*model, *version)
			if result != nil {
				fmt.Printf("Match Found:\nName: %s\nDescription: %s\n", result.Name, result.Description)
			} else {
				fmt.Println("No match found for the given model and version.")
			}

		} else {
			result := findOSByModelOrVersion(*model)
			if result != nil {
				fmt.Printf("Match Found:\nName: %s\nDescription: %s\n", result.Name, result.Description)
			} else {
				fmt.Println("No match found.")
			}
		}
	}

// mass get mac address list
        if *mode == "mac" && *mass {
                err := loadOSData()
                if err != nil {
                        fmt.Printf("Error loading OS data: %v\n", err)
                        return
                }
                inFile, err := os.Open("switches.txt")
                if err != nil {
                        fmt.Printf("error: %s\n", err)
                        os.Exit(1)
                }
                defer inFile.Close()
                devices := []string{}
                failed_devices := []string{}
                scanner := bufio.NewScanner(inFile)
                for scanner.Scan() {
                        line := scanner.Text()
                        res := strings.Split(line, " ")
                        if res[0] != "" || res[1] != "" || res[2] != "" {
                                h := res[0]
                                u := res[1]
                                p := res[2]
                                ipPort := fmt.Sprintf("%s:%d", h, *port)
                                fmt.Printf("Processing host: %s\n", ipPort)
                                brand, err := GetSSHBrand(u, p, ipPort)
                                if err != nil {
                                        fmt.Printf("GetSSHBrand err: %s\n", err)
                                        failed_devices = append(failed_devices, fmt.Sprintf("Failed detect brand on %s\n",h))
                                        continue
                                }
                                if brand == "" {
                                        failed_devices = append(failed_devices, fmt.Sprintf("Detected brand string is empty on %s\n",h))
                                        fmt.Printf("unknown model for host: %s\n", h)
                                        continue
                                } else {
                                        fmt.Printf("Device: %s OS is: %s\n",h, brand)
                                        os, err := ReturnOsInfo(brand)
                                        if err != nil {
                                             failed_devices = append(failed_devices, fmt.Sprintf("Cannot return os command for view mac addresses on %s\n",h))
                                             continue
                                        }
                                        result, err := RunCommands(u, p, ipPort, os.Pager, os.MacAddrComm)
                                        if err != nil {
                                             failed_devices = append(failed_devices, fmt.Sprintf("Cannot run command %s on %s\n",os.MacAddrComm,h))
                                             continue
                                        }
                                        err = SaveFile(fmt.Sprintf("macs/%s-%s.txt",h,os.Name),result)
                                        if err != nil {
                                             failed_devices = append(failed_devices, fmt.Sprintf("Unable save output file for command on %s\n",h))
                                        }
                             }
                        } else {
                                fmt.Printf("Corrupt line: %s\n", line)
                        }
                }
                // write about the problems in the file
                content := strings.Join(devices, "\n")
                SaveFile("fail.log", content)

        }


// mass detect
	if *mode == "detect" && *mass {
		err := loadOSData()
		if err != nil {
			fmt.Printf("Error loading OS data: %v\n", err)
			return
		}
		inFile, err := os.Open("switches.txt")
		if err != nil {
			fmt.Printf("error: %s\n", err)
			os.Exit(1)
		}
		defer inFile.Close()
		devices := []string{}
		scanner := bufio.NewScanner(inFile)
		for scanner.Scan() {
			line := scanner.Text()
			res := strings.Split(line, " ")
			if res[0] != "" || res[1] != "" || res[2] != "" {
				h := res[0]
				u := res[1]
				p := res[2]
				ipPort := fmt.Sprintf("%s:%d", h, *port)
				fmt.Printf("Processing host: %s\n", ipPort)
				brand, err := GetSSHBrand(u, p, ipPort)
				if err != nil {
					fmt.Printf("GetSSHBrand err: %s\n", err)
					continue
				}
				if brand == "" {
					fmt.Printf("unknown model for host: %s\n", h)
				} else {
					fmt.Printf("Device OS is: %s\n", brand)
					// add devices to the array and then save to file
					devices = append(devices, fmt.Sprintf("%s -> %s", h, brand))
				}
			} else {
				fmt.Printf("Corrupt line: %s\n", line)
			}
		}
		// write devices to file
		content := strings.Join(devices, "\n")
		SaveFile("detected_models.txt", content)

	}

        if *mode == "mac" && *user != "" && *pass != "" {
                err := loadOSData()
                if err != nil {
                        fmt.Printf("Error loading OS data: %v\n", err)
                        return
                }
                ipPort := fmt.Sprintf("%s:%d", *host, *port)
                brand, err := GetSSHBrand(*user, *pass, ipPort)
                if err != nil {
                        fmt.Printf("GetSSHBrand err: %s\n", err.Error())
                        os.Exit(1)
                }
                if brand == "" {
                        fmt.Printf("unknown model for host: %s\n", *host)
                } else {
                        fmt.Printf("Device OS is: %s\n", brand)
                        OS, err := ReturnOsInfo(brand)
                             if err != nil {
                             fmt.Printf("error: %s\n",err)
                        }
                        result, err := RunCommands(*user, *pass, ipPort, OS.MacAddrComm)
                        if err != nil {
                             fmt.Println("Error: %s\n", err.Error())
                             os.Exit(1)
                        }
                        fmt.Printf("%\n",result)

                }
                os.Exit(0)
        }

	if *mode == "detect" && *user != "" && *pass != "" {
		err := loadOSData()
		if err != nil {
			fmt.Printf("Error loading OS data: %v\n", err)
			return
		}
		ipPort := fmt.Sprintf("%s:%d", *host, *port)
		brand, err := GetSSHBrand(*user, *pass, ipPort)
		if err != nil {
			fmt.Printf("GetSSHBrand err: %s\n", err.Error())
			os.Exit(1)
		}
		if brand == "" {
			fmt.Printf("unknown model for host: %s\n", *host)
		} else {
			fmt.Printf("Device OS is: %s\n", brand)
		}
		os.Exit(0)
	}

	if *mode == "run" && *user != "" && *pass != "" {
		ipPort := fmt.Sprintf("%s:%d", *host, *port)
		brand, err := GetSSHBrand(*user, *pass, ipPort)
		if err != nil {
			fmt.Printf("GetSSHBrand err: %s\n", err.Error())
			os.Exit(1)
		}
		fmt.Printf("Device brand is: %s\n", brand)

		if *dump != "" {
			result, err := RunCommands(*user, *pass, ipPort, "show "+*dump)
			if err != nil {
				fmt.Println("RunCommands err:\n", err.Error())
				os.Exit(1)
			}
			fmt.Printf("OUTPUT: \n----------------------------------------\n%s\n--------------------------------------\n", result)
		}
		if *save != "" {
			result, err := RunCommands(*user, *pass, ipPort, "show "+*save)
			if err != nil {
				fmt.Println("RunCommands err:\n", err.Error())
				os.Exit(1)
			}
			fmt.Printf("Sanitizing output...\n")
			err, out := SanitizeConfigOutput(result)
			if err != nil {
				fmt.Printf("Unable to sanitize output!\n")
				os.Exit(1)
			}
			err = SaveFile(fmt.Sprintf("dump_%s.txt", *save), out)
			if err != nil {
				fmt.Printf("error when saving file for command: %s\n", *save)
			}
		}
	}

}
