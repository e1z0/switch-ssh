package main

import (
	"fmt"
	"strings"
	"time"
)

// DEPRECATED
const (
	HUAWEI          = "huawei"
	H3C             = "h3c"
	CISCO_SM        = "Version: 2.5" // cisco small business like SG350-28MP
	CISCO_SM1       = "SW version"   // cisco small business like SG500-52
	CISCO_SM2       = "Version: 3.3" // cisco small business like CBS350-48P-4G
	CISCO           = "Cisco IOS"
	CISCO_IOS_XE    = "Cisco IOS XE"
	CISCO_IOS_XR    = "Cisco IOS XR"
	CISCO_NX        = "NX-OS"
	CISCO_AIROS     = "Cisco Wireless Controller"
	CISCO_XE_SD     = "Cisco IOS XE SD-WAN"
	CISCO_STAR_OS   = "StarOS"
	CISCO_NX_ACI    = "ACI Software"
	CISCO_CATOS     = "Cisco Catalyst"
	CISCO_FXOS      = "FXOS"
	CISCO_ASAOS     = "Cisco Adaptive Security Appliance Software"
	CISCO_UCS       = "Cisco UCS Manager"
	ARUBA           = "ArubaOS ("
	ARUBA_INSTANT   = "InstantOS"
	ARUBA_CX        = "ArubaOS-CX"
	ARUBA_AOS_MC    = "Aruba Mobility Controller"
	ARUBA_AIRWAVE   = "AirWave"
	ARUBA_AOS_W     = "ArubaOS for Wireless"
	ARUBA_CLEARPASS = "ClearPass"
)

/**
 * Unified method for external calls, which completes the process of obtaining a session
 * (if it does not exist, a connection and session will be created and stored in the cache),
 * executing commands, and returning execution results.
 *
 * @param user     SSH connection username
 * @param password Password
 * @param ipPort   Switch IP and port
 * @param cmds     Commands to execute (can be multiple)
 * @return         Execution output and execution errors
 * @author shenbowei
 */
func RunCommands(user, password, ipPort string, cmds ...string) (string, error) {
	sessionKey := user + "_" + password + "_" + ipPort
	sessionManager.LockSession(sessionKey)
	defer sessionManager.UnlockSession(sessionKey)

	sshSession, err := sessionManager.GetSession(user, password, ipPort, "")
	if err != nil {
		LogError("GetSession error:%s", err)
		return "", err
	}
	sshSession.WriteChannel(cmds...)
	result := sshSession.ReadChannelTiming(2 * time.Second)
	filteredResult := filterResult(result, cmds[0])
	return filteredResult, nil
}

/**
 * Unified method for external calls, which completes the process of obtaining a session
 * (if it does not exist, a connection and session will be created and stored in the cache),
 * executing commands, and returning execution results.
 *
 * @param user     SSH connection username
 * @param password Password
 * @param ipPort   Switch IP and port
 * @param brand    Switch brand (can be empty)
 * @param cmds     Commands to execute (can be multiple)
 * @return         Execution output and execution errors
 * @author shenbowei
 */
func RunCommandsWithBrand(user, password, ipPort, brand string, cmds ...string) (string, error) {
	sessionKey := user + "_" + password + "_" + ipPort
	sessionManager.LockSession(sessionKey)
	defer sessionManager.UnlockSession(sessionKey)

	sshSession, err := sessionManager.GetSession(user, password, ipPort, brand)
	if err != nil {
		LogError("GetSession error:%s", err)
		return "", err
	}
	sshSession.WriteChannel(cmds...)
	result := sshSession.ReadChannelTiming(2 * time.Second)
	filteredResult := filterResult(result, cmds[0])
	return filteredResult, nil
}

/**
 * Unified method for external calls to obtain the switch model.
 *
 * @param user     SSH connection username
 * @param password Password
 * @param ipPort   Switch IP and port
 * @return         Device brand (huawei, h3c, cisco, "") and execution errors
 * @author shenbowei
 */
func GetSSHBrand(user, password, ipPort string) (string, error) {
	sessionKey := user + "_" + password + "_" + ipPort
	sessionManager.LockSession(sessionKey)
	defer sessionManager.UnlockSession(sessionKey)

	sshSession, err := sessionManager.GetSession(user, password, ipPort, "")
	if err != nil {
		LogError("GetSession error:%s", err)
		return "", err
	}
	return sshSession.GetSSHBrand(), nil
}

/**
 * Filters the execution results of the switch.
 *
 * @param result   Returned execution result (may contain unwanted data)
 * @param firstCmd The first executed command
 * @return         Filtered execution result
 * @author shenbowei
 */
func filterResult(result, firstCmd string) string {
	// Process the result and extract the part after the command
	filteredResult := ""
	resultArray := strings.Split(result, "\n")
	findCmd := false
	promptStr := ""
	for _, resultItem := range resultArray {
		resultItem = strings.Replace(resultItem, " \b", "", -1)
		if findCmd && (promptStr == "" || strings.Replace(resultItem, promptStr, "", -1) != "") {
			filteredResult += resultItem + "\n"
			continue
		}
		if strings.Contains(resultItem, firstCmd) {
			findCmd = true
			promptStr = resultItem[0:strings.Index(resultItem, firstCmd)]
			promptStr = strings.Replace(promptStr, "\r", "", -1)
			promptStr = strings.TrimSpace(promptStr)
			LogDebug("Find promptStr='%s'", promptStr)
			// Add the command to the result
			filteredResult += resultItem + "\n"
		}
	}
	if !findCmd {
		return result
	}
	return filteredResult
}

func LogDebug(format string, a ...interface{}) {
	if IsLogDebug {
		fmt.Println("[DEBUG]:" + fmt.Sprintf(format, a...))
	}
}

func LogError(format string, a ...interface{}) {
	fmt.Println("[ERROR]:" + fmt.Sprintf(format, a...))
}
