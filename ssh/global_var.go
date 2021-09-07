package ssh

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type TerminalModesConfig struct {
	ECHO          uint32 `mapstructure:"ECHO"`
	TTY_OP_ISPEED uint32 `mapstructure:"TTY_OP_ISPEED"`
	TTY_OP_OSPEED uint32 `mapstructure:"TTY_OP_OSPEED"`
}

type SSHConfig struct {
	Ciphers        []string            `mapstructure:"Ciphers"`
	KeyExchanges   []string            `mapstructure:"KeyExchanges"`
	Timeout        int                 `mapstructure:"Timeout"`
	TerminalModes  TerminalModesConfig `mapstructure:"TerminalModes"`
	Expects        []string            `mapstructure:"Expects"`
	LogOutputPaths []string            `mapstructure:"LogOutputPaths"`
}

var (
	GlobalSSHConfig *SSHConfig = &SSHConfig{}
)

func InitSSHConfig() {
	sshConfigFile := "conf/ssh_conf.yaml"
	v := viper.New()
	v.SetConfigFile(sshConfigFile)
	err := v.ReadInConfig()
	if err != nil {
		zap.S().Panic(err)
	}
	if err = v.Unmarshal(GlobalSSHConfig); err != nil {
		zap.S().Panic(err)
	}
}
