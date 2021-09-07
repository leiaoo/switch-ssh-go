package ssh

import (
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"net"
	"strings"
	"time"
)

type SSHSession struct {
	session     *ssh.Session
	in          chan string
	out         chan string
	brand       string
	lastUseTime time.Time
}

func NewSSHSession(user, password, ipPort string) (*SSHSession, error) {
	sshSession := new(SSHSession)
	if err := sshSession.createConnection(user, password, ipPort); err != nil {
		zap.S().Errorf("session 创建链接错误: %s", err.Error())
		return nil, err
	}
	if err := sshSession.muxShell(); err != nil {
		zap.S().Errorf("ewSSHSession muxShell error:%s", err.Error())
		return nil, err
	}
	if err := sshSession.start(); err != nil {
		zap.S().Errorf("NewSSHSession start error:%s", err.Error())
		return nil, err
	}
	sshSession.lastUseTime = time.Now()
	sshSession.brand = ""
	return sshSession, nil
}

func (s *SSHSession) createConnection(user, password, ipPort string) error {
	zap.S().Debugf("<Test> Begin connect: %s", ipPort)
	client, err := ssh.Dial("tcp", ipPort, &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Timeout: time.Duration(GlobalSSHConfig.Timeout) * time.Second,
		Config: ssh.Config{
			Ciphers: GlobalSSHConfig.Ciphers,
			KeyExchanges: GlobalSSHConfig.KeyExchanges,
		},
	})
	if err != nil {
		zap.S().Errorf("SSH 拨号错误: %s--, %s", err.Error(), ipPort)
		return err
	}
	zap.S().Debug("拨号结束")
	zap.S().Debug("开始创建新 session")
	session, err := client.NewSession()
	if err != nil {
		zap.S().Errorf("创建新session错误：%s", err.Error())
	}
	s.session = session
	zap.S().Debug("完成session创建")
	return nil
}

func (s *SSHSession) muxShell() error {
	defer func() {
		if err := recover(); err != nil {
			zap.S().Errorf("SSHSession muxShell 错误:%s", err)
		}
	}()
	modes := ssh.TerminalModes{
		ssh.ECHO:          GlobalSSHConfig.TerminalModes.ECHO,
		ssh.TTY_OP_OSPEED: GlobalSSHConfig.TerminalModes.TTY_OP_OSPEED,
		ssh.TTY_OP_ISPEED: GlobalSSHConfig.TerminalModes.TTY_OP_ISPEED,
	}
	if err := s.session.RequestPty("vt100", 80, 40, modes); err != nil {
		zap.S().Errorf("RequestPty 错误: %s", err.Error())
		return err
	}
	w, err := s.session.StdinPipe()
	if err != nil {
		zap.S().Errorf("StdinPipe() 错误: %s", err.Error())
		return err
	}
	r, err := s.session.StdoutPipe()
	if err != nil {
		zap.S().Errorf("StdoutPipe() 错误: %s", err.Error())
		return err
	}
	in := make(chan string, 1024)
	out := make(chan string, 1024)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				zap.S().Errorf("Goroutine muxShell wirte 错误: %s", err)
			}
		}()
		for cmd := range in {
			_, err := w.Write([]byte(cmd + "\n"))
			if err != nil {
				zap.S().Debugf("Writer write err: %s", err.Error())
				return
			}
		}
	}()
	go func() {
		defer func() {
			if err := recover(); err != nil {
				zap.S().Errorf("Goroutine muxShell read 错误: %s", err)
			}
		}()
		var (
			buf [65 * 1024]byte
			t   int
		)
		for {
			n, err := r.Read(buf[t:])
			if err != nil {
				zap.S().Debugf("Reader read err: %s", err.Error())
				return
			}
			t += n
			out <- string(buf[:t])
			t = 0
		}
	}()
	s.in = in
	s.out = out
	return nil
}

func (s *SSHSession) start() error {
	if err := s.session.Shell(); err != nil {
		zap.S().Errorf("Start shell error:%s", err.Error())
		return err
	}
	s.ReadChannelExpect(time.Second, GlobalSSHConfig.Expects...)
	return nil
}

func (s *SSHSession) ReadChannelExpect(timeout time.Duration, expects ...string) string {
	zap.S().Debugf("ReadChannelExpect <wait timeout = %d>", timeout/time.Millisecond)
	output := ""
	isDelayed := false
	for i := 0; i < 300; i++ {
		time.Sleep(time.Millisecond * 100)
		newData := s.readChannelData()
		zap.S().Debugf("ReadChannelExpect: read chanel buffer: %s", newData)
		if newData != "" {
			output += newData
			isDelayed = false
			continue
		}
		for _, expect := range expects {
			if strings.Contains(output, "The password needs to be changed. Change now? [Y/N]:N"){
				goto NoChangePassword
			}
			if strings.Contains(output, "The password needs to be changed. Change now? [Y/N]"){
				s.WriteChannel("N")
			}
			NoChangePassword:
			if strings.Contains(output, expect) {
				return output
			}
		}
		if !isDelayed {
			zap.S().Debugf("ReadChannelExpect: delay for timeout: %s", timeout)
			time.Sleep(timeout)
			isDelayed = true
		} else {
			return output
		}
	}
	return output
}

func (s *SSHSession) readChannelData() string {
	output := ""
	for {
		time.Sleep(time.Millisecond * 100)
		select {
		case channelData, ok := <-s.out:
			if !ok {
				return output
			}
			output += channelData
		default:
			return output
		}
	}
}

func (s *SSHSession) Close() {
	defer func() {
		if err := recover(); err != nil {
			zap.S().Errorf("SSHSession Close err:%s", err)
		}
	}()
	if err := s.session.Close(); err != nil {
		zap.S().Errorf("Close session err:%s", err.Error())
	}
	close(s.in)
	close(s.out)
}

func (s *SSHSession) CheckSelf() bool {
	defer func() {
		if err := recover(); err != nil {
			zap.S().Errorf("SSHSession CheckSelf err:%s", err)
		}
	}()
	s.WriteChannel("\n")
	result := s.ReadChannelExpect(2*time.Second, GlobalSSHConfig.Expects...)
	if strings.Contains(result, "#") ||
		strings.Contains(result, ">") ||
		strings.Contains(result, "]") {
		return true
	}
	return false
}

func (s *SSHSession) WriteChannel(cmds ...string) {
	zap.S().Debugf("WriteChannel <cmds=%v>", cmds)
	for _, cmd := range cmds {
		s.in <- cmd
	}
}

func (s *SSHSession) GetSSHBrand() string {
	defer func() {
		if err := recover(); err != nil {
			zap.S().Errorf("SSHSession GetSSHBrand err:%s", err)
		}
	}()
	if s.brand != "" {
		return s.brand
	}
	s.WriteChannel("dis version", "show version", "             ")
	result := s.ReadChannelTiming(time.Second)
	result = strings.ToLower(result)
	if strings.Contains(result, HUAWEI) {
		zap.S().Debug("The switch brand is <huawei>.")
		s.brand = HUAWEI
	} else if strings.Contains(result, H3C) {
		zap.S().Debug("The switch brand is <h3c>.")
		s.brand = H3C
	} else if strings.Contains(result, CISCO) {
		zap.S().Debug("The switch brand is <cisco>.")
		s.brand = CISCO
	}
	return s.brand
}

func (s *SSHSession) ReadChannelTiming(timeout time.Duration) string {
	zap.S().Debugf("ReadChannelTiming <wait timeout = %d>", timeout/time.Millisecond)
	output := ""
	isDelayed := false
	for i := 0; i < 300; i++ {
		time.Sleep(time.Millisecond * 100)
		newData := s.readChannelData()
		if newData != "" {
			output += newData
			isDelayed = false
			continue
		}
		if !isDelayed {
			zap.S().Debug("ReadChannelTiming: delay for timeout.")
			time.Sleep(timeout)
			isDelayed = true
		} else {
			return output
		}
	}
	return output
}
