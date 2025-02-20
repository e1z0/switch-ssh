package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"net"
	"strings"
	"time"
)

/**
 * Encapsulated SSH session, including the native ssh.Session and its standard input/output pipelines,
 * while also recording the last usage time.
 *
 * @attr session      Native SSH session
 * @attr in          Pipeline bound to the session's standard input
 * @attr out         Pipeline bound to the session's standard output
 * @attr lastUseTime Last usage time
 * @author shenbowei
 */
type SSHSession struct {
	session     *ssh.Session
	in          chan string
	out         chan string
	brand       string
	lastUseTime time.Time
}

/**
 * Creates an SSHSession, equivalent to the constructor of SSHSession.
 *
 * @param user     SSH connection username
 * @param password Password
 * @param ipPort   Switch IP and port
 * @return         Opened SSHSession and execution errors
 * @author shenbowei
 */
func NewSSHSession(user, password, ipPort string) (*SSHSession, error) {
	sshSession := new(SSHSession)
	if err := sshSession.createConnection(user, password, ipPort); err != nil {
		LogError("NewSSHSession createConnection error:%s", err.Error())
		return nil, err
	}
	if err := sshSession.muxShell(); err != nil {
		LogError("NewSSHSession muxShell error:%s", err.Error())
		return nil, err
	}
	if err := sshSession.start(); err != nil {
		LogError("NewSSHSession start error:%s", err.Error())
		return nil, err
	}
	sshSession.lastUseTime = time.Now()
	sshSession.brand = ""
	return sshSession, nil
}

/**
 * Retrieves the last usage time.
 *
 * @return time.Time
 * @author shenbowei
 */
func (this *SSHSession) GetLastUseTime() time.Time {
	return this.lastUseTime
}

/**
 * Updates the last usage time.
 *
 * @author shenbowei
 */
func (this *SSHSession) UpdateLastUseTime() {
	this.lastUseTime = time.Now()
}

/**
 * Connects to the switch and opens an SSH session.
 *
 * @param user     SSH connection username
 * @param password Password
 * @param ipPort   Switch IP and port
 * @return         Execution errors
 * @author shenbowei
 */
func (this *SSHSession) createConnection(user, password, ipPort string) error {
	LogDebug("<Test> Begin connect")
	client, err := ssh.Dial("tcp", ipPort, &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Timeout: 20 * time.Second,
		Config: ssh.Config{
			Ciphers: []string{"aes128-ctr", "aes192-ctr", "aes256-ctr", "aes128-gcm@openssh.com",
				"arcfour256", "arcfour128", "aes128-cbc", "aes256-cbc", "3des-cbc", "des-cbc",
			},
			KeyExchanges: []string{
				"diffie-hellman-group-exchange-sha1",
				"diffie-hellman-group-exchange-sha256",
				"diffie-hellman-group14-sha1",
				"ecdh-sha2-nistp256",
				"ecdh-sha2-nistp384",
				"ecdh-sha2-nistp521",
			},
		},
	})
	if err != nil {
		LogError("SSH Dial err:%s", err.Error())
		return err
	}
	LogDebug("<Test> End connect")
	LogDebug("<Test> Begin new session")
	session, err := client.NewSession()
	if err != nil {
		LogError("NewSession err:%s", err.Error())
		return err
	}
	this.session = session
	LogDebug("<Test> End new session")
	return nil
}

/**
 * Starts multiple threads to transfer data from the two returned pipes to the session's input and output pipes.
 *
 * @return Error information (error)
 * @author shenbowei
 */
func (this *SSHSession) muxShell() error {
	defer func() {
		if err := recover(); err != nil {
			LogError("SSHSession muxShell err:%s", err)
		}
	}()
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
	if err := this.session.RequestPty("vt100", 80, 40, modes); err != nil {
		LogError("RequestPty error:%s", err)
		return err
	}
	w, err := this.session.StdinPipe()
	if err != nil {
		LogError("StdinPipe() error:%s", err.Error())
		return err
	}
	r, err := this.session.StdoutPipe()
	if err != nil {
		LogError("StdoutPipe() error:%s", err.Error())
		return err
	}

	in := make(chan string, 1024)
	out := make(chan string, 1024)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				LogError("Goroutine muxShell write err:%s", err)
			}
		}()
		for cmd := range in {
			_, err := w.Write([]byte(cmd + "\n"))
			if err != nil {
				LogDebug("Writer write err:%s", err.Error())
				return
			}
		}
	}()

	go func() {
		defer func() {
			if err := recover(); err != nil {
				LogError("Goroutine muxShell read err:%s", err)
			}
		}()
		var (
			buf [65 * 1024]byte
			t   int
		)
		for {
			n, err := r.Read(buf[t:])
			if err != nil {
				LogDebug("Reader read err:%s", err.Error())
				return
			}
			t += n
			out <- string(buf[:t])
			t = 0
		}
	}()
	this.in = in
	this.out = out
	return nil
}

/**
 * Starts opening a remote SSH login shell, after which commands can be executed.
 *
 * @return Error information (error)
 * @author shenbowei
 */
func (this *SSHSession) start() error {
	if err := this.session.Shell(); err != nil {
		LogError("Start shell error:%s", err.Error())
		return err
	}
	// Wait for login information output
	this.ReadChannelExpect(time.Second, "#", ">", "]")
	return nil
}

/**
 * Checks if the current session is available.
 *
 * @return true: Available, false: Not available
 * @author shenbowei
 */
func (this *SSHSession) CheckSelf() bool {
	defer func() {
		if err := recover(); err != nil {
			LogError("SSHSession CheckSelf err:%s", err)
		}
	}()

	this.WriteChannel("\n")
	result := this.ReadChannelExpect(2*time.Second, "#", ">", "]")
	if strings.Contains(result, "#") ||
		strings.Contains(result, ">") ||
		strings.Contains(result, "]") {
		return true
	}
	return false
}

/**
 * Retrieves the brand of the switch currently accessed via SSH.
 *
 * @return string (huawei, h3c, cisco)
 * @author shenbowei
 */
func (this *SSHSession) GetSSHBrand() string {
	defer func() {
		if err := recover(); err != nil {
			LogError("SSHSession GetSSHBrand err:%s", err)
		}
	}()
	if this.brand != "" {
		return this.brand
	}
	// After displaying the version, add an extra set of spaces to avoid pagination issues,
	// where the first character of the pagination command becomes invalid due to too much version information.
	this.WriteChannel("dis version", "     ", "show version", "     ", "show inventory", "     ", "show system", "     ")
	result := this.ReadChannelTiming(3 * time.Second)
	//result = strings.ToLower(result)
	detect := verifyModelAndVersion(result, result)
	if detect != nil {
		fmt.Printf("Match Found:\nName: %s\nDescription: %s\n", detect.Name, detect.Description)
		this.brand = detect.Name
	} else {
		//fmt.Println("No match found for the given model and version.")
		AppendFile("unknown_models/data.txt", fmt.Sprintf("----------------BEGIN---------------\n%s\n--------------------------END---------------------\n", result))
	}

	return this.brand
}

/**
 * Closes the SSHSession, shutting down the session and input/output pipelines.
 *
 * @author shenbowei
 */
func (this *SSHSession) Close() {
	defer func() {
		if err := recover(); err != nil {
			LogError("SSHSession Close err:%s", err)
		}
	}()
	if err := this.session.Close(); err != nil {
		LogError("Close session err:%s", err.Error())
	}
	close(this.in)
	close(this.out)
}

/**
 * Writes execution commands to the pipeline.
 *
 * @param cmds... Commands to execute (multiple commands allowed)
 * @author shenbowei
 */
func (this *SSHSession) WriteChannel(cmds ...string) {
	LogDebug("WriteChannel <cmds=%v>", cmds)
	for _, cmd := range cmds {
		this.in <- cmd
	}
}

/**
 * Reads the execution results returned by the device from the output pipeline.
 * If the output stream interval exceeds the timeout or contains characters from expects, it will return.
 *
 * @param timeout Time to wait when no data is received from the device (if the timeout is exceeded, it is considered that the device's response has been fully read)
 * @param expects... Expected characters (can be multiple), returns when any of these are found
 * @return The result read from the output pipeline
 * @author shenbowei
 */
func (this *SSHSession) ReadChannelExpect(timeout time.Duration, expects ...string) string {
	LogDebug("ReadChannelExpect <wait timeout = %d>", timeout/time.Millisecond)
	output := ""
	isDelayed := false
	for i := 0; i < 300; i++ { // Read from the device a maximum of 300 times to avoid the method not returning
		time.Sleep(time.Millisecond * 100) // Sleep for 0.1 seconds each time to allow data in the out pipeline to accumulate for a while,
		// avoiding prematurely triggering the default wait exit.
		newData := this.readChannelData()
		LogDebug("ReadChannelExpect: read chanel buffer: %s", newData)
		if newData != "" {
			output += newData
			isDelayed = false
			continue
		}
		for _, expect := range expects {
			if strings.Contains(output, expect) {
				return output
			}
		}
		// If it has already waited once before, exit directly; otherwise, wait for a timeout once and then read the content again.
		if !isDelayed {
			LogDebug("ReadChannelExpect: delay for timeout")
			time.Sleep(timeout)
			isDelayed = true
		} else {
			return output
		}
	}
	return output
}

/**
 * Reads the execution results returned by the device from the output pipeline.
 * If the output stream interval exceeds the timeout, it will return.
 *
 * @param timeout Time to wait when no data is received from the device (if the timeout is exceeded, it is considered that the device's response has been fully read)
 * @return The result read from the output pipeline
 * @author shenbowei
 */
func (this *SSHSession) ReadChannelTiming(timeout time.Duration) string {
	LogDebug("ReadChannelTiming <wait timeout = %d>", timeout/time.Millisecond)
	output := ""
	isDelayed := false

	for i := 0; i < 300; i++ { // Read from the device a maximum of 300 times to avoid the method not returning.
		time.Sleep(time.Millisecond * 100) // Sleep for 0.1 seconds each time to allow data in the out pipeline to accumulate
		// preventing premature triggering of the default wait exit.
		newData := this.readChannelData()
		LogDebug("ReadChannelTiming: read chanel buffer: %s", newData)
		if newData != "" {
			output += newData
			isDelayed = false
			continue
		}
		// If it has already waited once, exit directly; otherwise, wait for a timeout once and then read the content again.
		if !isDelayed {
			LogDebug("ReadChannelTiming: delay for timeout.")
			time.Sleep(timeout)
			isDelayed = true
		} else {
			return output
		}
	}
	return output
}

/**
 * Clears the contents of the pipe buffer to prevent any leftover data from the previous read
 * from affecting the results of the next operation.
 */
func (this *SSHSession) ClearChannel() {
	//time.Sleep(time.Millisecond * 100)
	this.readChannelData()
}

/**
 * Clears the contents of the pipe buffer to avoid leftover data from the previous read
 * affecting the results of the next operation.
 */
func (this *SSHSession) readChannelData() string {
	output := ""
	for {
		time.Sleep(time.Millisecond * 100)
		select {
		case channelData, ok := <-this.out:
			if !ok {
				// If the out pipe is already closed, stop reading; otherwise, <-this.out will enter an infinite loop.
				return output
			}
			output += channelData
		default:
			return output
		}
	}
}
