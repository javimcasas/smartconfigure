# SmartConfigure

Aplicación de escritorio para lanzar el mismo bloque de comandos SSH sobre
muchos equipos de red a la vez, sustituyendo variables `{VARIABLE}` por los
valores de cada fila de un Excel.

No depende de ningún servicio en la nube: se ejecuta en local, con acceso
directo por SSH a los equipos de tu red.

## Cómo usarlo (interfaz gráfica)

Al abrir el ejecutable (doble clic) se abre una ventana simple (en inglés).
Solo necesitas traer el template; el Excel te lo genera la propia app:

1. **Choose template (.txt)** — el bloque de comandos, con `{VARIABLE}` donde
   cambie por equipo. Ver ejemplo en [`example/template.txt`](example/template.txt).
   Nada más elegirlo, la ventana muestra cuántas líneas y variables tiene.
2. **Generate Excel** — crea `devices.xlsx` junto al template (si ya existe,
   `devices-2.xlsx`, etc.; nunca sobrescribe) con la fila de cabecera lista:
   - Columna 1: `IP Address` (IP de gestión)
   - Columna 2: `Username`
   - Columna 3: `Password`
   - A partir de la columna 4: una columna por cada variable del template,
     con el nombre exacto de la variable (sin las llaves).
   Además lleva una hoja *Instructions* con las reglas. **Open in Excel** lo
   abre; rellena una fila por equipo y guarda. Si ya tienes un Excel con
   esa misma forma, **Choose Excel (.xlsx)** lo usa directamente.
3. Marca o desmarca **Dry-run** (por defecto activado: valida sin conectar
   a ningún equipo).
4. **▶ Run**. La app vuelve a leer el Excel del disco en cada ejecución, así
   que basta con guardar en Excel y pulsar Run otra vez. El progreso aparece
   en pantalla, y al terminar puedes abrir directamente la carpeta con los
   logs y el reporte (**Open output folder**).

### Exportar un script para SecureCRT

Si desde tu puesto solo puedes llegar a los equipos con **SecureCRT**, el
botón **Export SecureCRT script** genera en `smartconfigure-output/` dos
ficheros equivalentes, `smartconfigure-securecrt.vbs` (Windows) y
`smartconfigure-securecrt.py`, con el template ya renderizado para cada
fila del Excel. SmartConfigure **no se conecta a nada** al exportar. Luego,
en SecureCRT: *Script → Run…* → elige el `.vbs`. El script abre una sesión
SSH2 por equipo, envía las líneas esperando el prompt (`>` o `]`) tras cada
una, deja un `<IP>.securecrt.log` junto al script y sigue con el siguiente
equipo aunque uno falle. Al terminar muestra un resumen.

El script contiene los usuarios y contraseñas del Excel: trátalo como el
propio Excel y no lo subas a ningún sitio. Si alguna fila tiene una
variable sin valor, no se exporta nada (mismo criterio que el dry-run).

Ejemplo de cabecera de Excel para el template incluido:

| IP Address | Username | Password | SWITCH_NAME | SSH_PASSWORD | GATEWAY_IP | MGMT_IP | SUBNET_MASK |
|---|---|---|---|---|---|---|---|

## También funciona como línea de comandos

Para uso avanzado o scripts, los mismos flags de siempre siguen
funcionando exactamente igual (si se pasan `--template`/`--excel`, se salta
la interfaz gráfica):

```bash
smartconfigure --template template.txt --generate-excel auto   # crea devices.xlsx junto al template
smartconfigure --template template.txt --excel devices.xlsx --dry-run
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
| `--generate-excel` | — | Escribe un Excel de equipos vacío para `--template` en esa ruta y sale; `auto` = `devices.xlsx` junto al template |
| `--export-securecrt` | `false` | Escribe el script SecureCRT (`.vbs` + `.py`) en `--out` y sale, sin conectar a nada |
| `--port` | `22` | Puerto SSH |
| `--connect-timeout` | `10s` | Timeout de conexión SSH |
| `--idle-timeout` | `800ms` | Cuánto esperar en silencio antes de mandar la siguiente línea |
| `--line-timeout` | `15s` | Tope máximo de espera por línea |
| `--version` | — | Muestra la versión y sale |

Si no se pasa ningún flag de archivo, se abre la interfaz gráfica.

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
Debian/Ubuntu: `sudo apt install gcc libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev wayland-protocols`.

Con eso listo:

```bash
go mod tidy
go build -o smartconfigure .
```

Los tests (los de `internal/gui` usan el driver de test de Fyne — sin
pantalla, pero sí necesitan el compilador de C):

```bash
go test ./...
```

El generador de scripts SecureCRT (`internal/securecrt`) se comprueba
contra ficheros golden en `internal/securecrt/testdata/`; tras un cambio
intencionado del script, regenera y revisa el diff con
`go test ./internal/securecrt -update`.

El icono de la app vive en `assets/icon.svg` (fuente) y se rasteriza a
`Icon.png` (raíz, lo usa `fyne-cross -icon` en el workflow para el `.exe`)
y a `internal/gui/icon.png` (embebido: icono de ventana y barra de tareas).
Si cambias el SVG, regenera los dos PNG a 512 px.

## Descargar un binario ya compilado

Cada versión publicada (tag `vX.Y.Z`) genera automáticamente binarios para
Windows, Linux y macOS (Intel y Apple Silicon) vía GitHub Actions, y quedan
publicados en [Releases](../../releases). El enlace de "última versión" no
cambia nunca de URL:

```
https://github.com/javimcasas/smartconfigure/releases/latest/download/smartconfigure-windows-amd64.exe
```

## Landing web y visor de logs

La carpeta `public/` es la web que Cloudflare sirve en
`https://smartconfigure.hubsmartmatrix.com`: descargas, formato de
entrada, flags, y un **visor de logs** (`logs.html`) que parsea en el
navegador los `.log` y `report.csv` de una ejecución sin subir nada. Como el
resto de apps de SmartMatrix, `src/index.js` es un Worker que solo hace el
handoff SSO (`?sso=` → cookie `sc_session`, 24h) y sirve los assets; sin
sesión redirige al hub. Se despliega solo con Workers Builds en cada push a
`main`; necesita el secreto `SSO_SHARED_SECRET` (el mismo de
`smartmatrix-auth`) puesto con `npx wrangler secret put SSO_SHARED_SECRET`.

El diseño (tokens, componentes, reglas por superficie, incluida la ventana
Fyne) está documentado en `design-system/smartconfigure/` — leer
`MASTER.md` antes de tocar cualquier UI. Las URLs de descarga de
`public/index.html` deben coincidir con los nombres de asset que genera
`.github/workflows/release.yml`.

## Seguridad

Las contraseñas del Excel se leen solo en memoria durante la ejecución y no
se guardan en ningún sitio salvo en el propio `.xlsx` que tú mantienes. Los
logs de sesión (`smartconfigure-output/*.log`) sí contienen los comandos
enviados —incluida la contraseña SSH que se configura en el equipo si tu
template la manda en texto—, así que trátalos como material sensible y no
los subas a ningún repositorio.
