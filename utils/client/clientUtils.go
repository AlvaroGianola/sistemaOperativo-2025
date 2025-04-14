package clientUtils

import (
	//"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
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

func ObtenerIPLocal() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80") // No se conecta, solo sirve para obtener la IP local usada
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}
