#!/bin/bash

# Compila todos los módulos del TP y genera los binarios
# Asegurate de estar en la raíz del proyecto

MODULOS=("kernel" "cpu" "io" "memoria")

for modulo in "${MODULOS[@]}"; do
    echo "🛠 Compilando módulo: $modulo..."
    cd "$modulo"
    go build -o "$modulo" "$modulo.go"
    if [ $? -eq 0 ]; then
        echo "✅ $modulo compilado correctamente."
    else
        echo "❌ Error compilando $modulo"
    fi
    cd ..
    echo "--------------------------------------"

done

echo "🏁 Todos los módulos fueron compilados."