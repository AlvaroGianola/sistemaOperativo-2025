package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	ioGlobalUtils "github.com/sisoputnfrba/tp-golang/io/globalsIO"
	ioUtils "github.com/sisoputnfrba/tp-golang/io/ioUtilis"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

func main() {
	
	// Carga la configuración
	ioGlobalUtils.IoConfig = ioUtils.IniciarConfiguracion("config.json")

	// Verifica argumentos
	args := os.Args
	if len(args) < 2 {
		fmt.Println("Error: se debe pasar el nombre del dispositivo IO como argumento")
		os.Exit(1)
	}
	ioUtils.Nombre = args[1]

	// Inicializa el logger
	clientUtils.ConfigurarLogger("io"+ ioUtils.Nombre + ".log")


	// Encuentra un puerto libre
	puertoLibre, err := clientUtils.EncontrarPuertoDisponible(ioGlobalUtils.IoConfig.IPIo, ioGlobalUtils.IoConfig.PortIO)
	if err != nil {
		panic(err)
	}

	// Handshake al Kernel
	ioUtils.EnviarHandshakeAKernel(ioUtils.Nombre, puertoLibre)

	mux := http.NewServeMux()
	mux.HandleFunc("/recibirPeticion", ioUtils.RecibirPeticion)

	direccion := fmt.Sprintf("%s:%d", ioGlobalUtils.IoConfig.IPIo, puertoLibre)
	fmt.Printf("[IO] Servidor iniciado en puerto %d para dispositivo %s\n", puertoLibre, ioUtils.Nombre)

	// Capturar señales SIGINT y SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Avisar al Kernel si se recibe una señal
	go func() {
		<-sigs
		fmt.Println("[IO] Señal de apagado recibida. Avisando al Kernel...")
		ioUtils.AvisarDesconexion()
		os.Exit(0)
	}()

	http.ListenAndServe(direccion, mux)
}
