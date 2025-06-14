package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	memoriaUtils "github.com/sisoputnfrba/tp-golang/memoria/memoriaUtils"
)

type Paquete struct {
	Valores []string `json:"valores"`
}

func main() {
	http.HandleFunc("/obtenerEntradaTabla", entradaTabla)
	http.HandleFunc("/consultarMarco", consultarMarco)
	http.HandleFunc("/readMemoria", readMemoria)
	http.HandleFunc("/writeMemoria", writeMemoria)
	http.HandleFunc("/readPagina", readPagina)
	http.HandleFunc("/obtenerTamPagina", tamPagina)
	http.HandleFunc("/siguienteInstruccion", memoriaUtils.SiguienteInstruccion)
	http.HandleFunc("/iniciarProceso", memoriaUtils.IniciarProceso)
	http.HandleFunc("/finalizarProceso", memoriaUtils.FinalizarProceso)


	log.Println("🔧 Mock Memoria corriendo en :8002")
	http.ListenAndServe(":8002", nil)
}

func entradaTabla(w http.ResponseWriter, r *http.Request) {
	var p Paquete
	_ = json.NewDecoder(r.Body).Decode(&p)

	nivel, _ := strconv.Atoi(p.Valores[2])
	entrada, _ := strconv.Atoi(p.Valores[3])

	id := (nivel * 100) + entrada
	w.Write([]byte(fmt.Sprintf("%d", id)))
}

func consultarMarco(w http.ResponseWriter, r *http.Request) {
	var p Paquete
	_ = json.NewDecoder(r.Body).Decode(&p)

	pagina, _ := strconv.Atoi(p.Valores[1])
	w.Write([]byte(fmt.Sprintf("%d", 1000+pagina))) // Marco ficticio
}

func readMemoria(w http.ResponseWriter, r *http.Request) {
	var p Paquete
	_ = json.NewDecoder(r.Body).Decode(&p)

	tam, _ := strconv.Atoi(p.Valores[2])
	contenido := "X"
	result := ""
	for i := 0; i < tam; i++ {
		result += contenido
	}
	w.Write([]byte(result))
}

func writeMemoria(w http.ResponseWriter, r *http.Request) {
	var p Paquete
	_ = json.NewDecoder(r.Body).Decode(&p)

	log.Printf("WRITE: PID %s Marco %s Dato %s\n", p.Valores[0], p.Valores[1], p.Valores[2])
	w.Write([]byte("OK"))
}

func readPagina(w http.ResponseWriter, r *http.Request) {
	var p Paquete
	_ = json.NewDecoder(r.Body).Decode(&p)

	pagina := p.Valores[1]
	w.Write([]byte("ContenidoDePagina" + pagina))
}

func tamPagina(w http.ResponseWriter, r *http.Request) {
	// Devuelve [tamaño_pagina, niveles, entradas]
	w.Write([]byte(`[64, 2, 256]`)) // Simula un JSON array
}
