package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	ioGlobalUtils "github.com/sisoputnfrba/tp-golang/io/globalsIO"
	ioUtils "github.com/sisoputnfrba/tp-golang/io/ioUtilis"
	clientUtils "github.com/sisoputnfrba/tp-golang/utils/client"
)

func main() {
	// Carga la configuración
	ioGlobalUtils.IoConfig = ioUtils.IniciarConfiguracion("config.json")

	// Verifica argumentos
	if len(os.Args) < 2 {
		fmt.Println("Error: se debe pasar el nombre del dispositivo IO como argumento")
		os.Exit(1)
	}
	ioUtils.Nombre = os.Args[1]

	// Inicializa el logger
	clientUtils.ConfigurarLogger("io" + ioUtils.Nombre + ".log")

	// Encuentra un puerto libre y listener ya abierto
	listener, puertoLibre, err := clientUtils.EncontrarPuertoDisponible(ioGlobalUtils.IoConfig.IPIo, ioGlobalUtils.IoConfig.PortIO)
	if err != nil {
		panic(err)
	}
	fmt.Printf("[IO] Servidor iniciado en puerto %d para dispositivo %s\n", puertoLibre, ioUtils.Nombre)
	// Handshake al Kernel
	ioUtils.EnviarHandshakeAKernel(ioUtils.Nombre, puertoLibre)
	ioUtils.Puerto = strconv.Itoa(puertoLibre)
	// Registrar endpoint
	mux := http.NewServeMux()
	mux.HandleFunc("/recibirPeticion", ioUtils.RecibirPeticion)

	// Capturar señales SIGINT y SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		fmt.Println("[IO] Señal de apagado recibida. Avisando al Kernel...")
		ioUtils.AvisarDesconexion()
		os.Exit(0)
	}()

	// Servir usando el listener
	err = http.Serve(listener, mux)
	if err != nil {
		panic(err)
	}
}
