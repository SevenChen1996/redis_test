package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/set_magneti_levitation", SetMagneticLevitation)
	http.HandleFunc("/get_log_list", GetLogList)
	http.HandleFunc("/get_log_content",GetLogContent)
	http.HandleFunc("/set_fpga_bitstream",SetFpgaBitstream)
	err := http.ListenAndServe("0.0.0.0:8888", nil)
	if err != nil {
		log.Printf("err : %s", err.Error())
	}

}
