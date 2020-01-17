package main

import (
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
)

/******** local variables ***************/
const redisUnixDomainSockPath = "/tmp/redis.sock"

func getCurrentFuncName() string {
	pc := make([]uintptr, 1)
	runtime.Callers(2, pc)
	f := runtime.FuncForPC(pc[0])
	return f.Name()
}

func SetMagneticLevitation(responseWriter http.ResponseWriter, request *http.Request) {
	const (
		magneticLevitationWithAdjustment    uint8 = 0
		magneticLevitationWithoutAdjustment uint8 = 1
		magneticLevitationOscillation       uint8 = 2
	)

	type magneticLevitation struct {
		WorkMode uint8 `json:"work_mode"`
	}

	type magneticLevitationResponse struct {
		Result string `json:"result"`
	}

	if request.Method == "POST" {
		b, err := ioutil.ReadAll(request.Body)
		if err != nil {
			log.Printf("%s：read failed!", getCurrentFuncName())
			return
		}
		defer request.Body.Close()

		magneticLevitationData := &magneticLevitation{}
		err = json.Unmarshal(b, magneticLevitationData)
		if err != nil {
			log.Printf("%s:json prased failed!", getCurrentFuncName())
			return
		}

		client := redis.NewClient(&redis.Options{
			Addr: redisUnixDomainSockPath,
		})
		defer client.Close()

		_, err = client.Ping().Result()
		if err != nil {
			log.Printf("%s:connect to redis server failed!", getCurrentFuncName())
			return
		}

		client.Set("magnetic_levitation_work_mode", strconv.Itoa(int(magneticLevitationData.WorkMode)), 0)

		var result magneticLevitationResponse
		result.Result = "success"
		jsonData, _ := json.Marshal(result)
		_, err = io.WriteString(responseWriter, string(jsonData))
		if err != nil {
			log.Printf("%s: write failed!", getCurrentFuncName())
		}
	} else {
		fmt.Printf("%s only support method 'POST'", getCurrentFuncName())
		return
	}
}

func GetLogList(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method == "GET" {
		client := redis.NewClient(&redis.Options{
			Addr: redisUnixDomainSockPath,
		})
		defer client.Close()

		_, err := client.Ping().Result()

		if err != nil {
			log.Printf("%s:connect to redis server failed!", getCurrentFuncName())
			return
		}

		ret := client.SMembers("time_period_set")
		periodList, err := ret.Result()
		if err != nil {
			fmt.Printf("%s: read time_period_set failed!")
			return
		}

		jsonData, err := json.Marshal(periodList)
		if err != nil {
			fmt.Printf("%s: json.Marshal()  failed!")
			return
		}
		_, err = io.WriteString(responseWriter, string(jsonData))
		if err != nil {
			log.Printf("%s: write failed!", getCurrentFuncName())
		}
	} else {
		fmt.Printf("%s only support method 'GET'", getCurrentFuncName())
		return
	}
}

func GetLogContent(response http.ResponseWriter, request *http.Request) {
	type logTimePeriod struct {
		PeriodName string `json:"period_name"`
	}

	if request.Method == "GET" {
		b, err := ioutil.ReadAll(request.Body)
		if err != nil {
			log.Printf("%s：read failed!", getCurrentFuncName())
			return
		}
		defer request.Body.Close()

		logTimePeriodData := &logTimePeriod{}
		err = json.Unmarshal(b, logTimePeriodData)
		if err != nil {
			log.Printf("%s:json.Unmarshal() failed!", getCurrentFuncName())
			return
		}
		client := redis.NewClient(&redis.Options{
			Addr: redisUnixDomainSockPath,
		})
		defer client.Close()

		_, err = client.Ping().Result()
		if err != nil {
			log.Printf("%s:connect to redis server failed!", getCurrentFuncName())
			return
		}

		isMember, err := client.SIsMember("time_period_set", logTimePeriodData.PeriodName).Result()
		if !isMember || err != nil {
			log.Printf("%s:%s is not valid!", getCurrentFuncName(), logTimePeriodData.PeriodName)
			return
		}

		logMsgList, err := client.LRange(logTimePeriodData.PeriodName, 0, -1).Result()

		response.Header().Set("Content-Type", "text/plain")
		response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.log\"", logTimePeriodData.PeriodName))
		for _, v := range logMsgList {
			_, err = io.WriteString(response, v+"\r\n")
		}

	} else {
		fmt.Printf("%s only support method 'GET'", getCurrentFuncName())
		return
	}
}

//SetFpgaBitstream
var fpgaBitstreamDowloadMutex sync.Mutex

func SetFpgaBitstream(response http.ResponseWriter, request *http.Request) {
	type fpgaBitsteamInfo struct {
		PartialFlag   bool  `json:"partial_flag"`
		PartionNumber uint8 `json:"partion_number"`
	}
	const partionNumMax = 2
	readJsonFlag := false
	readFileFlag := false
	var fileName string

	if request.Method == "POST" {
		mpReader, err := request.MultipartReader()
		if err != nil {
			log.Printf("%s:request.MultipartReader() failed!", getCurrentFuncName())
			return
		}

		fpgaBitstreamInfoData := &fpgaBitsteamInfo{}
		var fileContent []byte
		for part, err := mpReader.NextPart(); err == nil; part, err = mpReader.NextPart() {
			if part.Header.Get("Content-Type") == "application/json" {
				jsonContent, err := ioutil.ReadAll(part)
				if err != nil {
					log.Printf("%s:ioutil.ReadAll failed!", getCurrentFuncName())
					return
				}
				part.Close()

				err = json.Unmarshal(jsonContent, fpgaBitstreamInfoData)
				if err != nil {
					log.Printf("%s:json.Unmarshal failed!", getCurrentFuncName())
					return
				}

				if fpgaBitstreamInfoData.PartionNumber >= partionNumMax {
					log.Printf("%s:fpgaBitstreamInfoData.PartionNumber is larger than partionNumMax(%d)!", getCurrentFuncName(), partionNumMax)
					return
				}

				readJsonFlag = true
			} else if part.Header.Get("Content-Type") == "application/octet-stream" {
				fileName = part.FileName()
				fileContent, err = ioutil.ReadAll(part)
				if err != nil {
					log.Printf("ioutil.ReadAll failed!")
					return
				}
				part.Close()
				readFileFlag = true
			}
		}

		if readJsonFlag && readFileFlag {
			file, err := ioutil.TempFile("", fileName)
			if err != nil {
				log.Printf("%s:create temp file!", getCurrentFuncName())
				return
			}
			defer os.Remove(file.Name())
			defer file.Close()

			_, err = file.Write(fileContent)
			if err != nil {
				log.Printf("%s:file.Writer failed!", getCurrentFuncName())
				return
			}
			err = file.Sync()
			if err != nil {
				log.Printf("%s:file.Sync failed!", getCurrentFuncName())
				return
			}


			fpgaBitstreamDowloadMutex.Lock()
			defer fpgaBitstreamDowloadMutex.Unlock()

			if fpgaBitstreamInfoData.PartialFlag {
				echoCmd := exec.Command("/bin/sh", "-c", "echo \"1\" > /sys/devices/soc0/amba/f8007000.devcfg/is_partial_bitstream")
				err = echoCmd.Run()
				if err != nil {
					log.Printf("%s:echoCmd.Run() failed!", getCurrentFuncName())
					return
				}
			}
			defer func() {
				if fpgaBitstreamInfoData.PartialFlag {
					echoCmd := exec.Command("/bin/sh", "-c", "echo \"0\" > /sys/devices/soc0/amba/f8007000.devcfg/is_partial_bitstream")
					err = echoCmd.Run()
					if err != nil {
						log.Printf("%s:echoCmd.Run() failed!", getCurrentFuncName())
						return
					}
				}
			}()

			//decoupler
			if fpgaBitstreamInfoData.PartialFlag {
				echoCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("echo on > /sys/class/pr_ctrl/pr_ctrl%d/decoupler_state",fpgaBitstreamInfoData.PartionNumber))
				err = echoCmd.Run()
				if err != nil {
					log.Printf("%s:echoCmd.Run() failed!", getCurrentFuncName())
					return
				}
			}

			defer func() {
				if fpgaBitstreamInfoData.PartialFlag {
					echoCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("echo off > /sys/class/pr_ctrl/pr_ctrl%d/decoupler_state",fpgaBitstreamInfoData.PartionNumber))
					err = echoCmd.Run()
					if err != nil {
						log.Printf("%s:echoCmd.Run() failed!", getCurrentFuncName())
						return
					}
				}
			}()

			copyToXdevcfgCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("cat %s > /dev/xdevcfg", file.Name()))
			err = copyToXdevcfgCmd.Run()
			if err != nil {
				log.Printf("%s:copyToXdevcfgCmd.Run() failed!", getCurrentFuncName())
				return
			}

		} else {
			log.Printf("%s:lack of parameter!", getCurrentFuncName())
			return
		}

	} else {
		log.Printf("%s only support method 'POST'", getCurrentFuncName())
		return
	}

}
