# SmartConfigure

Aplicación de escritorio para lanzar el mismo bloque de comandos SSH sobre
muchos equipos de red a la vez, sustituyendo variables `{VARIABLE}` por los
valores de cada fila de un Excel.

No depende de ningún servicio en la nube: se ejecuta en local, con acceso
directo por SSH a los equipos de tu red.

## Cómo usarlo (interfaz gráfica)

Al abrir el ejecutable (doble clic) se abre una ventana simple:

1. **Elegir template (.txt)** — el bloque de comandos, con `{VARIABLE}` donde
   cambie por equipo. Ver ejemplo en [`example/template.txt`](example/template.txt).
2. **Elegir Excel (.xlsx)** — una fila por equipo, con estas columnas:
   - Columna 1: `IP Equipo` (IP de gestión)
   - Columna 2: `Usuario`
   - Columna 3: `Contraseña`
   - A partir de la columna 4: una columna por cada variable del template,
     con el nombre exacto de la variable (sin las llaves).
3. Marca o desmarca **Dry-run** (por defecto activado: valida sin conectar
   a ningún equipo).
4. **▶ Ejecutar**. El progreso aparece en pantalla, y al terminar puedes
   abrir directamente la carpeta con los logs y el reporte.

Ejemplo de cabecera de Excel para el template incluido:

| IP Equipo | Usuario | Contraseña | SWITCH_NAME | SSH_PASSWORD | GATEWAY_IP | MGMT_IP | SUBNET_MASK |
|---|---|---|---|---|---|---|---|

## También funciona como línea de comandos

Para uso avanzado o scripts, los mismos flags de siempre siguen
funcionando exactamente igual (si se pasan `--template`/`--excel`, se salta
la interfaz gráfica):

```bash
smartconfigure --template template.txt --excel equipos.xlsx --dry-run
```

Por cada fila del Excel, SmartConfigure abre una sesión SSH interactiva,
manda cada línea del template (con las variables sustituidas) y guarda la
transcripción completa en `smartconfigure-output/<IP>.log`. Al terminar el
lote, deja un `smartconfigure-output/report.csv` con el resultado
(OK/FAILED) de cada equipo. Si una línea parece devolver un error de
sintaxis del propio equipo, esa fila se detiene ahí y se marca como FAILED;
el resto de equipos del lote continúa normalmente.

## Validar sin acceso a los equipos (dry-run)

Con la casilla **Dry-run** marcada (o `--dry-run` en CLI), SmartConfigure
sustituye las variables de cada fila y escribe en el log exactamente lo que
se habría mandado a cada equipo, sin abrir ninguna conexión SSH. Si a
alguna fila le falta el valor de una variable, se marca como FAILED con el
motivo exacto — así detectas errores de configuración antes de tocar
ningún equipo real.

## Flags de línea de comandos disponibles

| Flag | Por defecto | Descripción |
|---|---|---|
| `--template` | — | Ruta al archivo de template |
| `--excel` | — | Ruta al Excel de equipos |
| `--out` | `smartconfigure-output` | Carpeta de salida de logs y reporte |
| `--dry-run` | `false` | Valida template + Excel sin conectar a ningún equipo |
| `--port` | `22` | Puerto SSH |
| `--connect-timeout` | `10s` | Timeout de conexión SSH |
| `--idle-timeout` | `800ms` | Cuánto esperar en silencio antes de mandar la siguiente línea |
| `--line-timeout` | `15s` | Tope máximo de espera por línea |
| `--version` | — | Muestra la versión y sale |

Si no se pasa ni `--template` ni `--excel`, se abre la interfaz gráfica.

## Limitación conocida (v1)

La detección de "cuándo ha terminado un comando" se hace por silencio en el
buffer (idle timeout), no por reconocimiento del prompt real del equipo.
Funciona bien en la práctica para bloques de configuración normales, pero
si un comando pide una confirmación interactiva que no se anticipó, esa
fila puede quedarse esperando hasta el tope de tiempo y marcarse como
fallida. Revisa siempre el `.log` de un equipo que falle antes de relanzar.

## Compilar en local

La interfaz gráfica usa [Fyne](https://fyne.io), que necesita un compilador
de C (cgo) además de Go — a diferencia de una app de solo consola, esto es
obligatorio para compilar en tu máquina.

**Windows:** instala un toolchain de C, por ejemplo con
[MSYS2](https://www.msys2.org/) (`pacman -S mingw-w64-x86_64-toolchain`) y
añade `C:\msys64\mingw64\bin` al PATH.

**macOS:** ya tienes lo necesario si tienes Xcode Command Line Tools
(`xcode-select --install`).

**Linux:** instala las librerías de desarrollo de tu distro, por ejemplo en
Debian/Ubuntu: `sudo apt install gcc libgl1-mesa-dev xorg-dev`.

Con eso listo:

```bash
go mod tidy
go build -o smartconfigure .
```

## Descargar un binario ya compilado

Cada versión publicada (tag `vX.Y.Z`) genera automáticamente binarios para
Windows, Linux y macOS (Intel y Apple Silicon) vía GitHub Actions, y quedan
publicados en [Releases](../../releases). El enlace de "última versión" no
cambia nunca de URL:

```
https://github.com/javimcasas/smartconfigure/releases/latest/download/smartconfigure-windows-amd64.exe
```

## Seguridad

Las contraseñas del Excel se leen solo en memoria durante la ejecución y no
se guardan en ningún sitio salvo en el propio `.xlsx` que tú mantienes. Los
logs de sesión (`smartconfigure-output/*.log`) sí contienen los comandos
enviados —incluida la contraseña SSH que se configura en el equipo si tu
template la manda en texto—, así que trátalos como material sensible y no
los subas a ningún repositorio.
