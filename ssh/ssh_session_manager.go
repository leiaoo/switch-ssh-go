package ssh

import (
	"go.uber.org/zap"
	"sync"
	"time"
)

var (
	HuaweiNoPage = "screen-length 0 temporary"
	H3cNoPage    = "screen-length disable"
	CiscoNoPage  = "terminal length 0"
)

const (
	HUAWEI = "huawei"
	H3C    = "h3c"
	CISCO  = "cisco"
)

var sessionManager = NewSessionManager()

type SessionManager struct {
	sessionCache           map[string]*SSHSession
	sessionLocker          map[string]*sync.Mutex
	sessionCacheLocker     *sync.RWMutex
	sessionLockerMapLocker *sync.RWMutex
}

func NewSessionManager() *SessionManager {
	sessionManager := new(SessionManager)
	sessionManager.sessionCache = make(map[string]*SSHSession, 0)
	sessionManager.sessionLocker = make(map[string]*sync.Mutex, 0)
	sessionManager.sessionCacheLocker = new(sync.RWMutex)
	sessionManager.sessionLockerMapLocker = new(sync.RWMutex)
	sessionManager.RunAutoClean()
	return sessionManager
}

func (s *SessionManager) RunAutoClean() {
	go func() {
		for {
			timeoutSessionIndex := s.getTimeoutSessionIndex()
			s.sessionCacheLocker.Lock()
			for _, sessionKey := range timeoutSessionIndex {
				s.LockSession(sessionKey)
				delete(s.sessionCache, sessionKey)
				s.UnlockSession(sessionKey)
			}
			s.sessionCacheLocker.Unlock()
			time.Sleep(30 * time.Second)
		}
	}()
}

func (s *SessionManager) getTimeoutSessionIndex() []string {
	timeoutSessionIndex := make([]string, 0)
	s.sessionCacheLocker.RLock()
	defer func() {
		s.sessionCacheLocker.RUnlock()
		if err := recover(); err != nil {
			zap.S().Errorf("SSHSessionManager getTimeoutSessionIndex err:%s", err)
		}
	}()
	for sessionKey, sS := range s.sessionCache {
		timeDuratime := time.Now().Sub(sS.lastUseTime)
		if timeDuratime.Minutes() > 10 {
			zap.S().Debugf("RunAutoClean close session<%s, unuse time=%s>", sessionKey, timeDuratime.String())
			sS.Close()
			timeoutSessionIndex = append(timeoutSessionIndex, sessionKey)
		}
	}
	return timeoutSessionIndex
}

func (s *SessionManager) LockSession(sessionKey string) {
	s.sessionLockerMapLocker.RLock()
	mutex, ok := s.sessionLocker[sessionKey]
	s.sessionLockerMapLocker.RUnlock()
	if !ok {
		mutex = new(sync.Mutex)
		s.sessionLockerMapLocker.Lock()
		s.sessionLocker[sessionKey] = mutex
		s.sessionLockerMapLocker.Unlock()
	}
	mutex.Lock()
}

func (s *SessionManager) UnlockSession(sessionKey string) {
	s.sessionLockerMapLocker.RLock()
	s.sessionLocker[sessionKey].Unlock()
	s.sessionLockerMapLocker.RUnlock()
}

func (s *SessionManager) GetSession(user, password, ipPort, brand, sessionKey string) (*SSHSession, error) {
	session := s.GetSessionCache(sessionKey)
	if session != nil {
		if session.CheckSelf() {
			zap.S().Debug("-----GetSession from cache-----")
			session.lastUseTime = time.Now()
			return session, nil
		}
		zap.S().Debug("Check session failed")
	}
	if err := s.updateSession(user, password, ipPort, brand, sessionKey); err != nil {
		zap.S().Debugf("SSH session pool updateSession err:%s", err.Error())
		return nil, err
	} else {
		return s.GetSessionCache(sessionKey), nil
	}
}

func (s *SessionManager) updateSession(user, password, ipPort, brand, sessionKey string) error {
	mySession, err := NewSSHSession(user, password, ipPort)
	if err != nil {
		zap.S().Errorf("NewSSHSession err:%s", err.Error())
		return err
	}
	//初始化session，包括等待登录输出和禁用分页
	s.initSession(mySession, brand)
	//更新session的缓存
	s.SetSessionCache(sessionKey, mySession)
	return nil
}

func (s SessionManager) SetSessionCache(sessionKey string, session *SSHSession) {
	s.sessionCacheLocker.Lock()
	defer s.sessionCacheLocker.Unlock()
	s.sessionCache[sessionKey] = session
}

func (s SessionManager) initSession(session *SSHSession, brand string) {
	if brand != HUAWEI && brand != H3C && brand != CISCO {
		//如果传入的设备型号不匹配则自己获取
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
	default:
		return
	}
	session.ReadChannelExpect(time.Second, GlobalSSHConfig.Expects...)
}

func (s *SessionManager) GetSessionCache(sessionKey string) *SSHSession {
	s.sessionCacheLocker.RLock()
	defer s.sessionCacheLocker.RUnlock()
	cacheSession, ok := s.sessionCache[sessionKey]
	if ok {
		return cacheSession
	} else {
		return nil
	}
}
