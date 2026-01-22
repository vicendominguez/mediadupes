# Plan de Implementación - Herramienta de Deduplicación de Fotos/Videos (Paralela)

## Checklist de Tareas

### 1. Configuración Inicial
- [ ] Crear módulo Go (`go mod init photo-dedup`)
- [ ] Agregar dependencia `github.com/rwcarlsen/goexif/exif`
- [ ] Crear estructura básica del proyecto con `main.go`

### 2. Definir Estructuras de Datos
- [ ] Crear struct `FileInfo` con campos:
  - `Path string` (ruta completa)
  - `Size int64` (tamaño en bytes)
  - `HasExif bool` (si tiene datos EXIF)
  - `BaseName string` (nombre base sin extensión)
  - `IsVideo bool` (para omitir check de EXIF)

### 3. Implementar Recorrido Recursivo con Canal
- [ ] Usar `filepath.WalkDir` en goroutine separada
- [ ] Crear canal buffered `chan string` (capacidad ~1000)
- [ ] Enviar rutas de archivos válidos al canal
- [ ] Filtrar por extensiones:
  - Fotos: `.jpg`, `.jpeg`, `.png`, `.heic`, `.heif`
  - Videos: `.mp4`, `.mov`, `.avi`, `.mkv`, `.m4v`
- [ ] Cerrar canal al terminar recorrido

### 4. Implementar Detección de EXIF
- [ ] Crear función `hasExifData(filePath string) bool`
- [ ] Abrir archivo y usar `exif.Decode()`
- [ ] Retornar `false` para videos (sin intentar leer EXIF)
- [ ] Manejar errores silenciosamente (retornar `false`)

### 5. Implementar Worker Pool para Procesamiento Paralelo
- [ ] Crear `sync.WaitGroup` para workers
- [ ] Lanzar N goroutines workers (default: `runtime.NumCPU()`)
- [ ] Cada worker:
  - Lee ruta del canal
  - Extrae nombre base
  - Lee tamaño con `os.Stat()`
  - Detecta EXIF (solo si no es video)
  - Envía `FileInfo` a canal de resultados
- [ ] Canal de resultados: `chan FileInfo`

### 6. Implementar Deduplicación Thread-Safe
- [ ] Usar `sync.Mutex` para proteger mapa
- [ ] Goroutine recolectora lee canal de resultados
- [ ] Mapa `map[string]*FileInfo` (key = nombre base)
- [ ] Lógica de prioridad:
  1. Preferir el que tiene EXIF (solo fotos)
  2. Si empate en EXIF, preferir mayor tamaño
  3. Actualizar mapa si el nuevo es mejor
- [ ] Cerrar cuando workers terminen

### 7. Implementar Copia Paralela de Archivos
- [ ] Crear directorio destino si no existe
- [ ] Usar semáforo (buffered channel) para limitar concurrencia (ej: 4-8 copias simultáneas)
- [ ] Para cada archivo en mapa final:
  - Lanzar goroutine que:
    - Adquiere semáforo
    - Copia con `io.Copy`
    - Libera semáforo
- [ ] Usar `sync.WaitGroup` para esperar todas las copias

### 8. Configuración de Parámetros
- [ ] Agregar flags:
  - `-source` (directorio origen, default: `.`)
  - `-dest` (directorio destino, default: `fotos_exif`)
  - `-workers` (número de workers, default: `runtime.NumCPU()`)
  - `-copy-parallel` (copias simultáneas, default: `4`)
- [ ] Validar que directorio origen existe

### 9. Logging y Feedback
- [ ] Usar `sync/atomic` para contadores thread-safe:
  - Total archivos procesados
  - Total archivos únicos
  - Total archivos copiados
- [ ] Imprimir resumen al finalizar

### 10. Testing Básico
- [ ] Probar con estructura de directorios de prueba
- [ ] Verificar que prioriza correctamente
- [ ] Verificar que no hay race conditions (`go run -race`)

## Arquitectura de Concurrencia

```
[WalkDir] → [Canal Rutas] → [Worker Pool] → [Canal Resultados] → [Dedup Map] → [Copia Paralela]
              (buffered)      (N workers)      (buffered)          (mutex)        (semáforo)
```

## Notas de Implementación

- **Workers**: Usar `runtime.NumCPU()` como default
- **Buffered channels**: Capacidad ~1000 para rutas, ~100 para resultados
- **Semáforo de copia**: Limitar a 4-8 para no saturar disco
- **Race detector**: Probar con `go run -race` antes de release
- **Errores**: No detener ejecución por archivos individuales con error
