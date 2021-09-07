package ssh

import "go.uber.org/zap"

func NewLogger() (*zap.Logger, error){
	cfg := zap.NewDevelopmentConfig()
	cfg.OutputPaths = GlobalSSHConfig.LogOutputPaths
	return cfg.Build()
}

func InitLogger(){
	logger, err := NewLogger()
	if err != nil{
		panic(err)
	}
	zap.ReplaceGlobals(logger)
}