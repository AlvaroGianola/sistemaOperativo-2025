package main

import (
	"net/http"
	"os"

	ioUtils "github.com/sisoputnfrba/tp-golang/io/ioUtilis"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

func main() {
	clientUtils.ConfigurarLogger("io.log")

	ioUtils.IoConfig = ioUtils.IniciarConfiguracion("config.json")

	mux := http.NewServeMux()

	mux.HandleFunc("/recibirPeticion", ioUtils.RecibirPeticion)

	args := os.Args
	nombre := args[1]
	ioUtils.EnviarHandshakeAKernel(nombre)

	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		panic(err)
	}
}
