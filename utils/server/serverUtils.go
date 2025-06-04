package serverUtils

import (
	"encoding/json"
	"log"
	"net/http"
)

type Mensaje struct {
	Mensaje string `json:"mensaje"`
}

type Paquete struct {
	Valores []string `json:"valores"`
}

func RecibirPaquetes(w http.ResponseWriter, r *http.Request) Paquete {
	decoder := json.NewDecoder(r.Body)
	var paquete Paquete
	err := decoder.Decode(&paquete)
	if err != nil {
		log.Printf("error al decodificar mensaje: %s\n", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al decodificar mensaje"))
		return Paquete{}
	}

	return paquete
}

func RecibirMensaje(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var mensaje Mensaje
	err := decoder.Decode(&mensaje)
	if err != nil {
		log.Printf("Error al decodificar mensaje: %s\n", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Error al decodificar mensaje"))
		return
	}

	log.Println("Me llego un mensaje de un cliente")
	log.Printf("%+v\n", mensaje)

}
