package ssh

import (
	"fmt"
	"go.uber.org/zap"
	"strings"
	"time"
)

func RunCommands(user, password, ipPort string, cmds ...string) (string, error) {
	sessionKey := fmt.Sprintf("%s-%s-%s", user, password, ipPort)
	sessionManager.LockSession(sessionKey)
	defer sessionManager.UnlockSession(sessionKey)
	sshSession, err := sessionManager.GetSession(user, password, ipPort, "", sessionKey)
	if err != nil {
		zap.S().Errorf("GetSession error:%s", err)
		return "", err
	}
	sshSession.WriteChannel(cmds...)
	result := sshSession.ReadChannelTiming(2 * time.Second)
	filteredResult := filterResult(result, cmds[0])
	return filteredResult, nil
}

func RunCommandsWithBrand(user, password, ipPort, brand string, cmds ...string) (string, error) {
	sessionKey := user + "_" + password + "_" + ipPort
	sessionManager.LockSession(sessionKey)
	defer sessionManager.UnlockSession(sessionKey)
	sshSession, err := sessionManager.GetSession(user, password, ipPort, brand, sessionKey)
	if err != nil {
		zap.S().Errorf("GetSession error:%s", err.Error())
		return "", err
	}
	sshSession.WriteChannel(cmds...)
	result := sshSession.ReadChannelTiming(2 * time.Second)
	filteredResult := filterResult(result, cmds[0])
	return filteredResult, nil
}

func filterResult(result, firstCmd string) string {
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
			zap.S().Debugf("Find promptStr='%s'", promptStr)
			filteredResult += resultItem + "\n"
		}
	}
	if !findCmd {
		return result
	}
	return filteredResult
}

func init() {
	InitSSHConfig()
	InitLogger()
}
