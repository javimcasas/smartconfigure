# SmartConfigure

Herramienta de escritorio (línea de comandos) para lanzar el mismo bloque de
comandos SSH sobre muchos equipos de red a la vez, sustituyendo variables
`{VARIABLE}` por los valores de cada fila de un Excel.

No depende de ningún servicio en la nube: se ejecuta en local, con acceso
directo por SSH a los equipos de tu red.

## Cómo funciona

1. Escribes un **template** en un `.txt` con las líneas de comandos que se
   mandarían a mano por SSH, usando `{NOMBRE_VARIABLE}` donde cambie por
   equipo. Ver ejemplo en [`example/template.txt`](example/template.txt).
2. Preparas un **Excel** (`.xlsx`) con una fila por equipo:
   - Columna 1: `IP Equipo` (IP de gestión)
   - Columna 2: `Usuario`
   - Columna 3: `Contraseña`
   - A partir de la columna 4: una columna por cada variable usada en el
     template, con el nombre exacto de la variable (sin las llaves).

   Ejemplo de cabecera para el template incluido:

   | IP Equipo | Usuario | Contraseña | SWITCH_NAME | SSH_PASSWORD | GATEWAY_IP | MGMT_IP | SUBNET_MASK |
   |---|---|---|---|---|---|---|---|

3. Ejecutas:

   ```bash
   smartconfigure --template template.txt --excel equipos.xlsx
   ```

Por cada fila del Excel, SmartConfigure abre una sesión SSH interactiva,
manda cada línea del template (con las variables ya sustituidas) y guarda
la transcripción completa en `smartconfigure-output/<IP>.log`. Al terminar
el lote, deja un `smartconfigure-output/report.csv` con el resultado
(OK/FAILED) de cada equipo.

Si una línea parece devolver un error de sintaxis del propio equipo, esa
fila se detiene ahí (no sigue mandando el resto del template a un equipo en
mal estado) y se marca como FAILED en el reporte; el resto de equipos del
lote continúa normalmente.

## Validar sin acceso a los equipos (`--dry-run`)

Si todavía no tienes acceso a la red de gestión, puedes validar que el
template y el Excel encajan bien sin conectar a nada:

```bash
smartconfigure --template template.txt --excel equipos.xlsx --dry-run
```

En modo `--dry-run`, SmartConfigure sustituye las variables de cada fila y
escribe en el log exactamente lo que se habría mandado a cada equipo, pero
sin abrir ninguna conexión SSH. Si a alguna fila le falta el valor de una
variable (columna mal escrita en el Excel, por ejemplo), esa fila se marca
como FAILED con el motivo exacto — así detectas errores de configuración
antes de tocar ningún equipo real.

## Flags disponibles

| Flag | Por defecto | Descripción |
|---|---|---|
| `--template` | — | Ruta al archivo de template (obligatorio) |
| `--excel` | — | Ruta al Excel de equipos (obligatorio) |
| `--out` | `smartconfigure-output` | Carpeta de salida de logs y reporte |
| `--dry-run` | `false` | Valida template + Excel sin conectar a ningún equipo |
| `--port` | `22` | Puerto SSH |
| `--connect-timeout` | `10s` | Timeout de conexión SSH |
| `--idle-timeout` | `800ms` | Cuánto esperar en silencio antes de mandar la siguiente línea |
| `--line-timeout` | `15s` | Tope máximo de espera por línea |
| `--version` | — | Muestra la versión y sale |

## Limitación conocida (v1)

La detección de "cuándo ha terminado un comando" se hace por silencio en el
buffer (idle timeout), no por reconocimiento del prompt real del equipo.
Funciona bien en la práctica para bloques de configuración normales, pero
si un comando pide una confirmación interactiva que no se anticipó (un
`y/n` inesperado), esa fila puede quedarse esperando hasta `--line-timeout`
y marcar la fila como fallida. Revisa siempre el `.log` de un equipo que
falle antes de relanzar.

## Compilar en local

Requiere Go 1.22+:

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
