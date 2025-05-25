package clientUtils

import (
	//"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
)

type Mensaje struct {
	Mensaje string `json:"mensaje"`
}

type Paquete struct {
	Valores []string `json:"valores"`
}

var Logger *slog.Logger

/*
func LeerConsola() {
	var valores []string

	log.Println("Ingrese los mensajes")
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	for text != "\n" {
		valores = append(valores, text)
		log.Print(text)
		text, _ = reader.ReadString('\n')
	}

	GenerarYEnviarPaquete(valores)
}*/

func GenerarYEnviarPaquete(valores []string, ip string, puerto int, direccion string) {
	// Leemos y cargamos el paquete
	paquete := Paquete{Valores: valores}

	log.Printf("paqute a enviar: %+v", paquete)
	// Enviamos el paqute
	EnviarPaquete(ip, puerto, direccion, paquete)
}

func EnviarMensaje(ip string, puerto int, mensajeTxt string) {
	mensaje := Mensaje{Mensaje: mensajeTxt}
	body, err := json.Marshal(mensaje)
	if err != nil {
		log.Printf("error codificando mensaje: %s", err.Error())
	}

	url := fmt.Sprintf("http://%s:%d/mensaje", ip, puerto)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("error enviando mensaje a ip:%s puerto:%d", ip, puerto)
	}

	log.Printf("respuesta del servidor: %s", resp.Status)
}

func EnviarPaquete(ip string, puerto int, direccion string, paquete Paquete) {
	body, err := json.Marshal(paquete)
	if err != nil {
		log.Printf("error codificando mensajes: %s", err.Error())
	}

	url := fmt.Sprintf("http://%s:%d/%s", ip, puerto, direccion)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("error enviando mensajes a ip:%s puerto:%d", ip, puerto)
	}

	log.Printf("respuesta del servidor: %s", resp.Status)
}

func ConfigurarLogger(nombreArchivo string) {
	archivo, err := os.Create(nombreArchivo)
	if err != nil {
		log.Fatal(err)
	}

	Logger = slog.New(slog.NewTextHandler(archivo, &slog.HandlerOptions{
		Level: slog.LevelDebug, // Nivel de log configurable si se desea
	}))

	Logger.Info("Logger inicializado para " + nombreArchivo)
}

// Busca el primer puerto disponible a partir de un puerto base
/*func EncontrarPuertoDisponible(ip string, puertoInicial int) (int, error) {
puerto := puertoInicial
for {
	addr := ip + ":" + strconv.Itoa(puerto)

	// Intentamos escuchar en esa dirección
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// Si da error, ese puerto está ocupado, pasamos al siguiente
		puerto++
		continue
	}

	// Si llegamos acá, el puerto está libre → lo liberamos
	_ = ln.Close()

	// Devolvemos ese puerto como disponible
	return puerto, nil
}}*/

func EncontrarPuertoDisponible(ip string, puertoInicial int) (net.Listener, int, error) {
	puerto := puertoInicial
	for {
		addr := ip + ":" + strconv.Itoa(puerto)

		ln, err := net.Listen("tcp", addr)
		if err != nil {
			puerto++
			continue
		}

		// NO cerramos ln → lo usamos
		return ln, puerto, nil
	}
}

func EnviarPaqueteConRespuesta(ip string, puerto int, direccion string, paquete Paquete) *http.Response {
	body, err := json.Marshal(paquete)
	if err != nil {
		log.Printf("error codificando mensajes: %s", err.Error())
		return nil
	}

	url := fmt.Sprintf("http://%s:%d/%s", ip, puerto, direccion)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("error enviando mensajes a ip:%s puerto:%d", ip, puerto)
		return nil
	}

	return resp
}

func EnviarPaqueteConRespuestaBody(ip string, puerto int, direccion string, paquete Paquete) []byte {
	body, err := json.Marshal(paquete)
	if err != nil {
		log.Printf("error codificando mensajes: %s", err.Error())
		return nil
	}

	url := fmt.Sprintf("http://%s:%d/%s", ip, puerto, direccion)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("error enviando mensajes a ip:%s puerto:%d", ip, puerto)
		return nil
	}
	defer resp.Body.Close()

	respuesta, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("error leyendo respuesta del body: %s", err.Error())
		return nil
	}

	return respuesta
}
