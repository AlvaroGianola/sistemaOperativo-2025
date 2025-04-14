package main

import (
	"net/http"

	"github.com/sisoputnfrba/tp-golang/memoria/memoriaUtils"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

func main() {

	clientUtils.ConfigurarLogger("memoria.log")

	memoriaUtils.MemoriaConfig = memoriaUtils.IniciarConfiguracion("config.json")

	mux := http.NewServeMux()

	mux.HandleFunc("/cpu", memoriaUtils.RecibirPeticionCpu)
	mux.HandleFunc("/kernel", memoriaUtils.RecibirPeticionKernel)

	err := http.ListenAndServe(":8082", mux)
	if err != nil {
		panic(err)
	}
}
