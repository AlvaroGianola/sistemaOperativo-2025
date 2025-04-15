#!/bin/bash

# Ruta base donde están los ejecutables precompilados
BASE_DIR="$(pwd)"

# Ejecutar cada módulo en una terminal separada usando binarios
# Asegurate de haber hecho `go build` en cada carpeta (ej: go build -o kernel kernel.go)

# Kernel
gnome-terminal -- bash -c "cd $BASE_DIR/kernel && ./kernel; exec bash"

# CPU (usa config y binario generado previamente)
gnome-terminal -- bash -c "cd $BASE_DIR/cpu && ./cpu; exec bash"

# IO (requiere pasar el nombre del dispositivo como argumento)
gnome-terminal -- bash -c "cd $BASE_DIR/io && ./io impresora1; exec bash"

# Memoria
gnome-terminal -- bash -c "cd $BASE_DIR/memoria && ./memoria; exec bash"

# Dar tiempo a que los servidores se levanten
sleep 3

# Simular envío de proceso a CPU (usa puerto configurado en config.json)
echo "Enviando proceso a CPU..."
curl -X POST -H "Content-Type: application/json" \
     -d '{ "pid": 1, "pc": 0 }' http://localhost:8081/recibirProceso

# Simular envío de petición a IO
echo "Enviando petición a IO..."
curl -X POST -H "Content-Type: application/json" \
     -d '{ "pid": 1, "tiempo": 3000 }' http://localhost:8080/recibirPeticion

# Simular mensaje a memoria desde CPU
echo "Petición CPU a Memoria..."
curl -X POST http://localhost:8082/cpu

# Simular mensaje a memoria desde Kernel
echo "Petición Kernel a Memoria..."
curl -X POST http://localhost:8082/kernel

# Fin del test
echo "✅ Test del Checkpoint 1 ejecutado. Revisá los logs para ver los resultados."
