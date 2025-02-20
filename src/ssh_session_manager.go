package main

import (
	"sync"
	"time"
)

var (
	HuaweiNoPage  = "screen-length 0 temporary"
	H3cNoPage     = "screen-length disable"
	CiscoNoPage   = "terminal length 0"
	CiscoSMNoPage = "terminal datadump"
	ArubaCXNoPage = "no page"
)

var sessionManager = NewSessionManager()

/**
 * Manages SSHSessions and caches opened sessions, automatically handling sessions that have not been used for more than 10 minutes.
 *
 * @attr sessionCache: Map that caches all opened sessions (used within the last 10 minutes)
 * @attr sessionLocker: Device lock
 * @attr globalLocker: Global lock
 * @author shenbowei
 */
type SessionManager struct {
	sessionCache           map[string]*SSHSession
	sessionLocker          map[string]*sync.Mutex
	sessionCacheLocker     *sync.RWMutex
	sessionLockerMapLocker *sync.RWMutex
}

/**
 * Creates a SessionManager, equivalent to the constructor of SessionManager.
 *
 * @return SessionManager instance
 * @author shenbowei
 */
func NewSessionManager() *SessionManager {
	sessionManager := new(SessionManager)
	sessionManager.sessionCache = make(map[string]*SSHSession, 0)
	sessionManager.sessionLocker = make(map[string]*sync.Mutex, 0)
	sessionManager.sessionCacheLocker = new(sync.RWMutex)
	sessionManager.sessionLockerMapLocker = new(sync.RWMutex)
	// Start an automatic cleanup thread to clear session cache not used for 10 minutes.
	sessionManager.RunAutoClean()
	return sessionManager
}

func (this *SessionManager) SetSessionCache(sessionKey string, session *SSHSession) {
	this.sessionCacheLocker.Lock()
	defer this.sessionCacheLocker.Unlock()
	this.sessionCache[sessionKey] = session
}

func (this *SessionManager) GetSessionCache(sessionKey string) *SSHSession {
	this.sessionCacheLocker.RLock()
	defer this.sessionCacheLocker.RUnlock()
	cache, ok := this.sessionCache[sessionKey]
	if ok {
		return cache
	} else {
		return nil
	}
}

/**
 * Locks the specified session.
 *
 * @param sessionKey: The index key of the session
 * @author shenbowei
 */
func (this *SessionManager) LockSession(sessionKey string) {
	this.sessionLockerMapLocker.RLock()
	mutex, ok := this.sessionLocker[sessionKey]
	this.sessionLockerMapLocker.RUnlock()
	if !ok {
		// If the lock cannot be obtained, it needs to be created. A global lock is required when updating the lock storage.
		mutex = new(sync.Mutex)
		this.sessionLockerMapLocker.Lock()
		this.sessionLocker[sessionKey] = mutex
		this.sessionLockerMapLocker.Unlock()
	}
	mutex.Lock()
}

/**
 * Unlocks the specified session.
 *
 * @param sessionKey: The index key of the session
 * @author shenbowei
 */
func (this *SessionManager) UnlockSession(sessionKey string) {
	this.sessionLockerMapLocker.RLock()
	this.sessionLocker[sessionKey].Unlock()
	this.sessionLockerMapLocker.RUnlock()
}

/**
 * Updates the session in the session cache, connects to the device, opens a session, initializes the session
 * (wait for login, identify device type, execute disable pagination), and adds it to the cache.
 *
 * @param user     SSH connection username
 * @param password Password
 * @param ipPort   Switch IP and port
 * @return         Execution errors
 * @author shenbowei
 */
func (this *SessionManager) updateSession(user, password, ipPort, brand string) error {
	sessionKey := user + "_" + password + "_" + ipPort
	mySession, err := NewSSHSession(user, password, ipPort)
	if err != nil {
		LogError("NewSSHSession err:%s", err.Error())
		return err
	}
	// Initializes the session, including waiting for login output and disabling pagination.
	this.initSession(mySession, brand)
	// Updates the session cache.
	this.SetSessionCache(sessionKey, mySession)
	return nil
}

/**
 * Initializes the session (wait for login, identify device type, execute disable pagination).
 *
 * @param session: The SSHSession that requires initialization
 * @author shenbowei
 */
func (this *SessionManager) initSession(session *SSHSession, brand string) {
	if brand != HUAWEI && brand != H3C && brand != CISCO {
		// If the provided device model does not match, it will fetch the model itself.
		brand = session.GetSSHBrand()
	}
	switch brand {
	case HUAWEI:
		session.WriteChannel(HuaweiNoPage)
		break
	case H3C:
		session.WriteChannel(H3cNoPage)
		break
	case CISCO:
		session.WriteChannel(CiscoNoPage)
		break
	case ARUBA_CX:
		session.WriteChannel(ArubaCXNoPage)
	case CISCO_SM:
		session.WriteChannel(CiscoSMNoPage)
	case CISCO_SM1:
		session.WriteChannel(CiscoSMNoPage)
	case CISCO_SM2:
		session.WriteChannel(CiscoSMNoPage)
	default:
		return
	}
	session.ReadChannelExpect(time.Second, "#", ">", "]")
}

/**
 * Retrieves the session from the cache. If it does not exist or is unavailable, it will be recreated.
 *
 * @param user     SSH connection username
 * @param password Password
 * @param ipPort   Switch IP and port
 * @return         SSHSession
 * @author shenbowei
 */
func (this *SessionManager) GetSession(user, password, ipPort, brand string) (*SSHSession, error) {
	sessionKey := user + "_" + password + "_" + ipPort
	session := this.GetSessionCache(sessionKey)
	if session != nil {
		// Before returning, verify if the session is available. If not, it must be recreated and the cache updated.
		if session.CheckSelf() {
			LogDebug("-----GetSession from cache-----")
			session.UpdateLastUseTime()
			return session, nil
		}
		LogDebug("Check session failed")
	}
	// If it does not exist or validation fails, a reconnection is required, and the cache should be updated.
	if err := this.updateSession(user, password, ipPort, brand); err != nil {
		LogError("SSH session pool updateSession err:%s", err.Error())
		return nil, err
	} else {
		return this.GetSessionCache(sessionKey), nil
	}
}

/**
 * Starts automatically cleaning up sessions in the cache that have not been used for more than 10 minutes.
 *
 * @author shenbowei
 */
func (this *SessionManager) RunAutoClean() {
	go func() {
		for {
			timeoutSessionIndex := this.getTimeoutSessionIndex()
			this.sessionCacheLocker.Lock()
			for _, sessionKey := range timeoutSessionIndex {
				//this.LockSession(sessionKey)
				delete(this.sessionCache, sessionKey)
				//this.UnlockSession(sessionKey)
			}
			this.sessionCacheLocker.Unlock()
			time.Sleep(30 * time.Second)
		}
	}()
}

/**
 * Retrieves all sessionKeys of sessions that have timed out (not used for more than 10 minutes) in the cache.
 *
 * @return []string Array of sessionKeys for all timed-out sessions
 * @author shenbowei
 */
func (this *SessionManager) getTimeoutSessionIndex() []string {
	timeoutSessionIndex := make([]string, 0)
	this.sessionCacheLocker.RLock()
	defer func() {
		this.sessionCacheLocker.RUnlock()
		if err := recover(); err != nil {
			LogError("SSHSessionManager getTimeoutSessionIndex err:%s", err)
		}
	}()
	for sessionKey, SSHSession := range this.sessionCache {
		timeDuratime := time.Now().Sub(SSHSession.GetLastUseTime())
		if timeDuratime.Minutes() > 10 {
			LogDebug("RunAutoClean close session<%s, unuse time=%s>", sessionKey, timeDuratime.String())
			SSHSession.Close()
			timeoutSessionIndex = append(timeoutSessionIndex, sessionKey)
		}
	}
	return timeoutSessionIndex
}
