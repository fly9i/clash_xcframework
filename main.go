package goClash

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation
#import <Foundation/Foundation.h>
#import "UIHelper.h"
*/
import "C"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/Dreamacro/clash/component/mmdb"
	"github.com/Dreamacro/clash/config"
	"github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/hub/executor"
	"github.com/Dreamacro/clash/hub/route"
	"github.com/Dreamacro/clash/log"
	"github.com/Dreamacro/clash/tunnel/statistic"
	"github.com/oschwald/geoip2-golang"
	"github.com/phayes/freeport"
)

var secretOverride string = ""

func IsAddrValid(addr string) bool {
	if addr != "" {
		comps := strings.Split(addr, ":")
		v := comps[len(comps)-1]
		if port, err := strconv.Atoi(v); err == nil {
			if port > 0 && port < 65535 {
				return CheckPortAvailable(port)
			}
		}
	}
	return false
}

func CheckPortAvailable(port int) bool {
	if port < 1 || port > 65534 {
		return false
	}
	addr := ":"
	l, err := net.Listen("tcp", addr+strconv.Itoa(port))
	if err != nil {
		log.Warnln("check port fail 0.0.0.0:%d", port)
		return false
	}
	_ = l.Close()

	addr = "127.0.0.1:"
	l, err = net.Listen("tcp", addr+strconv.Itoa(port))
	if err != nil {
		log.Warnln("check port fail 127.0.0.1:%d", port)
		return false
	}
	_ = l.Close()
	log.Infoln("check port %d success", port)
	return true
}

//export InitClashCore
func InitClashCore() {
	configFile := filepath.Join(constant.Path.HomeDir(), constant.Path.Config())
	constant.SetConfig(configFile)
}

func ReadConfig(path string) ([]byte, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, err
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("Configuration file %s is empty", path)
	}
	return data, err
}

func getRawCfg() (*config.RawConfig, error) {
	buf, err := ReadConfig(constant.Path.Config())
	if err != nil {
		return nil, err
	}
	return config.UnmarshalRawConfig(buf)
}

//export GetRawCfg
func GetRawCfg() (string, error) {
	buf, err := ReadConfig(constant.Path.Config())
	if err != nil {
		return "", err
	}
	conf,err := config.UnmarshalRawConfig(buf)
	if err != nil {
		return "", err
	}
	bytes,err := json.Marshal(conf)
	return string(bytes), err
}

func stringToRawConfig(s string) (*config.RawConfig, error) {
    var cfg config.RawConfig
    err := json.Unmarshal([]byte(s), &cfg)
    if err != nil {
        return nil, err
    }
    return &cfg, nil
}

func parseDefaultConfigThenStart(checkPort, allowLan bool, proxyPort int, externalController string) (*config.Config, error) {
	rawCfg, err := getRawCfg()
	
	if err != nil {
		return nil, err
	}

	if proxyPort > 0 {
		rawCfg.MixedPort = proxyPort
		if rawCfg.Port == rawCfg.MixedPort {
			rawCfg.Port = 0
		}
		if rawCfg.SocksPort == rawCfg.MixedPort {
			rawCfg.SocksPort = 0
		}
	} else {
		if rawCfg.MixedPort == 0 {
			if rawCfg.Port > 0 {
				rawCfg.MixedPort = rawCfg.Port
				rawCfg.Port = 0
			} else if rawCfg.SocksPort > 0 {
				rawCfg.MixedPort = rawCfg.SocksPort
				rawCfg.SocksPort = 0
			} else {
				rawCfg.MixedPort = 7890
			}

			if rawCfg.SocksPort == rawCfg.MixedPort {
				rawCfg.SocksPort = 0
			}

			if rawCfg.Port == rawCfg.MixedPort {
				rawCfg.Port = 0
			}
		}
	}
	if secretOverride != "" {
		rawCfg.Secret = secretOverride
	}
	rawCfg.ExternalUI = ""
	rawCfg.Profile.StoreSelected = false
	if len(externalController) > 0 {
		rawCfg.ExternalController = externalController
	}
	if checkPort {
		if !IsAddrValid(rawCfg.ExternalController) {
			port, err := freeport.GetFreePort()
			if err != nil {
				return nil, err
			}
			rawCfg.ExternalController = "127.0.0.1:" + strconv.Itoa(port)
			rawCfg.Secret = ""
		}
		rawCfg.AllowLan = allowLan

		if !CheckPortAvailable(rawCfg.MixedPort) {
			if port, err := freeport.GetFreePort(); err == nil {
				rawCfg.MixedPort = port
			}
		}
	}

	cfg, err := config.ParseRawConfig(rawCfg)
	if err != nil {
		return nil, err
	}
	go route.Start(cfg.General.ExternalController, cfg.General.Secret)
	executor.ApplyConfig(cfg, true)
	return cfg, nil
}

func ParseDefaultConfigThenStart(checkPort, allowLan bool, proxyPort int, externalController string) (string, error) {
	cfg,err := parseDefaultConfigThenStart(checkPort, allowLan, proxyPort, externalController)
	if err != nil {
		return "", err
	}
	bytes,err := json.Marshal(cfg)
	return string(bytes), err
}

//export VerifyClashConfig
func VerifyClashConfig(content string) string {

	b := []byte(content)
	cfg, err := executor.ParseWithBytes(b)
	if err != nil {
		return err.Error()
	}

	if len(cfg.Proxies) < 1 {
		return "No proxy found in config"
	}
	return "success"
}

//export ClashSetupLogger
func ClashSetupLogger() {
	sub := log.Subscribe()
	go func() {
		for elm := range sub {
			log := elm.(log.Event)
			cs := C.CString(log.Payload)
			cl := C.CString(log.Type())
			C.sendLogToUI(cs, cl)
			C.free(unsafe.Pointer(cs))
			C.free(unsafe.Pointer(cl))
		}
	}()
}

//export ClashSetupTraffic
func ClashSetupTraffic() {
	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		t := statistic.DefaultManager
		buf := &bytes.Buffer{}
		for range tick.C {
			buf.Reset()
			up, down := t.Now()
			C.sendTrafficToUI(C.longlong(up), C.longlong(down))
		}
	}()
}

// export Clash_checkSecret
func Clash_checkSecret() string {
	cfg, err := getRawCfg()
	if err != nil {
		return ""
	}

	if cfg.Secret != "" {
		return cfg.Secret
	}
	return ""
}

//export Clash_setSecret
func Clash_setSecret(secret string) {
	secretOverride = secret
}

func stringToConfig(s string) (*config.Config, error) {
    var cfg config.Config
    err := json.Unmarshal([]byte(s), &cfg)
    if err != nil {
        return nil, err
    }
    return &cfg, nil
}


//export Run
func Run(checkConfig, allowLan bool, portOverride int, externalController string) string {
	cfg, err := parseDefaultConfigThenStart(checkConfig, allowLan, portOverride, externalController)

	if err != nil {
		return err.Error()
	}

	portInfo := map[string]string{
		"externalController": cfg.General.ExternalController,
		"secret":             cfg.General.Secret,
	}

	jsonString, err := json.Marshal(portInfo)
	if err != nil {
		return err.Error()
	}

	return string(jsonString)
}

//export SetUIPath
func SetUIPath(path string) {
	route.SetUIPath(path)
}

func SetConfigPath(path string) {
	constant.SetConfig(path)
}

//export ClashUpdateConfig
func ClashUpdateConfig(path string) string {
	cfg, err := executor.ParseWithPath(path)
	if err != nil {
		return err.Error()
	}
	executor.ApplyConfig(cfg, false)
	return "success"
}

//export ClashGetConfigs
func ClashGetConfigs() string {
	general := executor.GetGeneral()
	jsonString, err := json.Marshal(general)
	if err != nil {
		return err.Error()
	}
	return string(jsonString)
}

func SetHomeDir(path string) {
	constant.SetHomeDir(path)
}

//export VerifyGEOIPDataBase
func VerifyGEOIPDataBase() bool {
	mmdb, err := geoip2.Open(constant.Path.MMDB())
	if err != nil {
		log.Warnln("mmdb fail:%s", err.Error())
		return false
	}

	_, err = mmdb.Country(net.ParseIP("114.114.114.114"))
	if err != nil {
		log.Warnln("mmdb lookup fail:%s", err.Error())
		return false
	}
	return true
}

//export Clash_getCountryForIp
func Clash_getCountryForIp(ip string) string {
	record, _ := mmdb.Instance().Country(net.ParseIP(ip))
	if record != nil {
		return record.Country.IsoCode
	}
	return ""
}

//export Clash_closeAllConnections
func Clash_closeAllConnections() {
	snapshot := statistic.DefaultManager.Snapshot()
	for _, c := range snapshot.Connections {
		c.Close()
	}
}

//export Clash_getProggressInfo
func Clash_getProggressInfo() string {
	return GetTcpNetList() + GetUDpList()
}
