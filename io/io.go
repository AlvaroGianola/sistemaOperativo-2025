package main

import (
	"fmt"
	"net/http"
	"os"

	ioGlobalUtils "github.com/sisoputnfrba/tp-golang/io/globalsIO"
	ioUtils "github.com/sisoputnfrba/tp-golang/io/ioUtilis"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

func main() {
	// Inicializa el logger para registrar eventos del módulo IO en un archivo
	clientUtils.ConfigurarLogger("io.log")

	// Carga la configuración desde el archivo config.json
	ioGlobalUtils.IoConfig = ioUtils.IniciarConfiguracion("config.json")

	// Verifica que se haya pasado un nombre como argumento
	args := os.Args
	
	nombre:= args[1]
	if len(args) < 2 {
		fmt.Println("Error: se debe pasar el nombre del dispositivo IO como argumento")
		os.Exit(1)
	}
	
	// Envia el handshake al Kernel informando el nombre del dispositivo IO
	ioUtils.EnviarHandshakeAKernel(nombre)

	// Inicializa el servidor HTTP y registra el endpoint que recibe peticiones del Kernel
	mux := http.NewServeMux()
	mux.HandleFunc("/recibirPeticion", ioUtils.RecibirPeticion)

	// Usa el puerto definido en el archivo de configuración
	direccion := fmt.Sprintf(":%d", ioGlobalUtils.IoConfig.PortIO)
	fmt.Printf("[IO] Servidor iniciado en puerto %d para dispositivo %s\n", ioGlobalUtils.IoConfig.PortIO, nombre)
	err := http.ListenAndServe(direccion, mux)
	if err != nil {
		panic(err)
	}
}
