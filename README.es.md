<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="frontend/public/appicon.png" />
    <img src="frontend/public/appicon-light.png" alt="lich" width="88" height="88" />
  </picture>
  <h1>lich</h1>
  <p><a href="README.md">English</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <strong>Español</strong> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.ru.md">Русский</a></p>
  <p><strong>Un ADE centrado en la terminal para los agentes de código que ya usas.</strong></p>
  <p>
    Abre tus proyectos, ejecuta agentes como Claude Code, Codex y opencode en
    terminales reales y mantén git (worktrees, diffs y pull requests) a la vista
    sin salir de la ventana. Un solo binario Go estático, sin Electron: la
    interfaz se abre en el Chromium embebido de lich en Linux, Windows y macOS.
  </p>
  <p><a href="https://omartelo.github.io/lich/"><strong>omartelo.github.io/lich</strong></a></p>
  <p>
    <a href="https://github.com/omartelo/lich/releases"><img alt="Release" src="https://img.shields.io/github/v/release/omartelo/lich?color=4285F4&label=release" /></a>
    <img alt="Go" src="https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white" />
    <img alt="Shell" src="https://img.shields.io/badge/shell-embedded%20Chromium%20(CEF)-4285F4?logo=googlechrome&logoColor=white" />
    <img alt="Platform" src="https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-333" />
    <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-AGPL--3.0-blue" /></a>
    <a href="https://github.com/sponsors/omartelo"><img alt="Sponsor" src="https://img.shields.io/github/sponsors/omartelo?color=ea4aaa&logo=githubsponsors&label=sponsors" /></a>
  </p>
  <img src="docs/media/session.png" alt="Cuatro sesiones de Claude Code lado a lado en un mismo grupo, cada una en su propio git worktree: la barra lateral las lista con su rama y su indicador de diff, y el pie de ventana muestra el modelo, el plan y la rama" width="1000" />
  <!-- sponsor-logos: company logos go here, between the screenshot and Why lich -->
</div>

## Por qué lich

lich te permite:

- **Ejecutar el agente que ya tienes.** [Claude Code](https://www.anthropic.com/claude-code),
  [Codex](https://github.com/openai/codex), Antigravity,
  [opencode](https://github.com/sst/opencode), oh-my-pi,
  [Crush](https://github.com/charmbracelet/crush), el
  [Cursor CLI](https://cursor.com/docs/cli) y el
  [Kiro CLI](https://kiro.dev/docs/cli/) se ejecutan aquí exactamente igual.
  Indica a lich una sola vez dónde está el binario de cada uno y luego elige el
  predeterminado o uno por sesión. Lo que lich puede leer *de vuelta* de una
  sesión cambia según el proveedor, y la tabla está en
  [Compatibilidad por proveedor](#compatibilidad-por-proveedor).
- **Tener una terminal de verdad.** Shells respaldados por PTY, varios por
  proyecto, renderizados en la GPU, con un historial de desplazamiento con
  búsqueda que sobrevive a una recarga completa de la página. Dale a una un
  punto de entrada (`lazygit`, `k9s`, `pnpm dev`) y se abre directamente en esa
  herramienta cada vez, igual en Linux, macOS y Windows: antes no se carga ningún
  rc del shell ni perfil de PowerShell, así que un punto de entrada que te
  funciona a ti también le funciona a quien lo compartas. El pie de ventana
  sigue los `cd` y muestra la rama; y, en una sesión de Claude Code, Codex o Kiro
  CLI, el modelo y la ventana de contexto en uso; en Claude Code, Codex,
  Antigravity, Cursor CLI y una suscripción de OpenCode Go, cuánto queda de la
  ventana móvil de tu plan; y, si lo pides, lo que ha gastado la sesión, en los
  proveedores que lo registran.
- **Poner una sesión a trabajar para otra.** Entrega una tarea a otra tarjeta y
  su propio agente escribe la respuesta de vuelta, sea lo que sea que se ejecute
  en cada extremo: el agente llega a las demás sesiones mediante herramientas
  que recibe al iniciarse (MCP en Claude Code y Codex) o que trae el plugin.
  Toda esa superficie funciona también como el comando `lich` en cualquier shell,
  `--json` incluido, de modo que un script puede dirigir una sesión sin ningún
  agente de por medio ([`docs/cli.md`](docs/cli.md)).
- **Vigilar varias a la vez.** Pon cualquier sesión en un grupo junto a aquella en
  la que estás y lich distribuye los paneles solo, según cuántos haya y cuánto
  espacio tenga la ventana: ocho quedan cuatro en fila en un monitor ultrapanorámico
  y dos en fila en un portátil. Una sesión que delegó trabajo en otras arma el
  grupo a su alrededor con un clic. Cada grupo tiene nombre, un proyecto conserva
  tantos como quieras, y cada uno recibe un bloque en la barra lateral que puedes
  plegar, renombrar o deshacer.
- **Ramificar un worktree sin la preparación.** Crea uno desde cualquier rama
  base y lich lo llena con tus archivos `.env*` ignorados por git, le asigna un
  puerto de servidor de desarrollo que ningún otro checkout ni proceso de la
  máquina esté usando, y ejecuta el script de preparación de tu proyecto antes de
  que arranque el agente. Si lo inicias desde el trabajo sin confirmar del
  checkout, esos cambios viajan con él, archivos sin seguimiento incluidos.
- **Ejecutar una sesión confinada.** Un agente puede abrirse dentro de un sandbox
  del sistema operativo: un home nuevo y vacío que contiene solo el estado propio
  de ese proveedor, el resto de la máquina en solo lectura y acceso de escritura
  al checkout para el que se abrió. Tus claves ssh, tus credenciales de la nube
  y todos los demás repositorios del disco sencillamente no están ahí: el
  contrapeso de dejar que un agente se salte las peticiones de permiso. Linux usa
  bubblewrap y macOS `sandbox-exec`; no es una barrera contra código hostil, y
  [`docs/ceilings.md`](docs/ceilings.md) dice qué es lo que no detiene.
- **Revisar el diff donde lo lees.** Un dock de CodeMirror muestra los cambios en
  curso junto a un árbol de archivos en vivo, pliega los que solo movieron
  espacios en blanco cuando se lo pides, y muestra una imagen o un PDF modificado
  antes y después. Haz clic derecho sobre una selección para comentar esas
  líneas; el lote se pega en la sesión como un único prompt, sin enviar. Su
  pestaña Código recorre el checkout y busca en el texto de sus archivos.
- **Entregar el pull request desde aquí.** Lista los pull requests abiertos del
  repositorio, haz checkout de uno en un worktree propio, y luego lee el diff,
  revísalo en línea y haz merge, con los métodos que la rama base realmente
  acepta. En una rama que todavía no tiene pull request, **Crear con el agente**
  le pide a la sesión de esa rama que abra uno, y no se envía nada hasta que
  pulses Enter.

Además: [temas](docs/themes.md) que importas como JSON o instalas desde un
repositorio git, una paleta de comandos `Ctrl`/`Cmd`+`K` que salta por nombre o
por lo que se dijo en la conversación, una notificación de escritorio cuando una
sesión te está esperando, y `lich rage` / `lich doctor` para cuando no arranca
en absoluto.

El desarrollo está activo: los bugs y las solicitudes de funciones van en
[Issues](https://github.com/omartelo/lich/issues), y lo que cambió en cada
versión está en [CHANGELOG.md](CHANGELOG.md).

## Compatibilidad por proveedor

Cada proveedor se inicia en una terminal real, se reanuda por id de conversación,
se confina con el mismo sandbox y recibe la misma forma de llegar a tus otras
sesiones. Lo que cambia es lo que lich puede *leer* de vuelta de una sesión,
porque lee lo que cada CLI deja escrito y no hay dos que anoten las mismas cosas.

| Lo que obtienes | Claude Code | Codex | Antigravity | opencode | oh-my-pi | Crush | Cursor CLI | Kiro CLI |
| --- | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: |
| Ventana de contexto en el pie de ventana | sí | sí | no | no | no | no | no | sí |
| Costo en el pie de ventana | sí | sí | no | sí | sí | sí | no | créditos |
| Cuánto queda de tu plan | sí | sí | sí | solo OpenCode Go | no | no | sí | no |
| Un turno que el límite de uso de tu plan detuvo se reanuda cuando el límite se restablece | sí | sí | no | sus propios reintentos | sus propios reintentos | no | no | no |
| Spinner mientras corre un turno, anillo cuando termina | sí | sí | sí | sí | sí | no | sí | sí |
| Un pull request que abre el agente aparece en su tarjeta al terminar el turno | sí | sí | sí | sí | sí | no, cuando la ventana recupera el foco | sí | sí |
| Se repinta con el aspecto nuevo al cambiar lich entre un tema claro y uno oscuro | sí | no | no | sí | sí | no | sí | no |
| Campana cuando el agente está bloqueado esperándote | sí | sí | no | sí | no | no | no | no |
| Ctrl+Shift+Y salta a la siguiente sesión que te espera | sí | sí | no | sí | no | no | no | no |
| La tarjeta avisa cuando la conversación se está compactando | sí | no | no | no | no | no | no | no |
| Filtro de la barra lateral por estado: En espera, En ejecución, Sin leer | sí | sí | no En espera | sí | no En espera | no, siempre Inactiva | no En espera | no En espera |
| Si tu agente lo ejecuta dentro de otra sesión (`claude -p`, `cursor-agent -p`), no ensucia la tarjeta de esa sesión | sí | sí | sí | sí | sí | sí | sí | no |
| La máquina no entra en reposo mientras corre un turno | sí | sí | sí | sí | sí | no | sí | sí |
| **Último turno** de la pestaña Revisión, con el resumen del agente | sí | sí | sí | sí | sí | no | sí | sí |
| Buscar en la conversación desde la paleta | sí | sí | sí | sí | sí | sí | no | sí |
| Reanudar una conversación iniciada fuera de lich, desde la pestaña Historial de la paleta | sí | sí | sí | sí | sí | Linux; macOS y Windows sin probar | sí | sí |
| Un pegado largo puede llegar como texto editable en vez de un marcador plegado (Ajustes › Proveedores, desactivado por defecto) | sí | no | no | sí | no medido | no, se convierte en un adjunto `paste_N.txt` | no | sí |
| Bifurcar una conversación en un nuevo worktree | sí | sí | no | sí | no | no | no | no |
| Abrir una sesión con un esfuerzo de razonamiento elegido | sí | sí | sí | no | sí | no | en el nombre del modelo | sí |
| Abrir una sesión con ultracode activado | sí | no | no | no | no | no | no | no |
| Varias cuentas lado a lado como perfiles, cada una con su propio inicio de sesión completo, y una sesión movida a otra cuenta que sigue en la misma tarjeta (en la ventana nativa, experimental: Ajustes › Proveedores) | sí | sí, con sus propios plugins en cada perfil y los ajustes de Personal compartidos por todos | no | sí, con sesión iniciada desde una tarjeta de terminal; una sesión no puede pasar a otro perfil, porque sus conversaciones están en la base de datos que guarda el inicio de sesión | sí, con sesión iniciada desde una tarjeta de terminal, con sus ajustes compartidos por todos los perfiles | no medido | Linux y Windows (en macOS su inicio de sesión está en el Llavero), con sesión iniciada desde una tarjeta de terminal, con `mcp.json`, hooks y skills compartidos por todos los perfiles | solo Linux (macOS guarda su inicio de sesión en el Llavero y en Windows nada lo mueve), con sesión iniciada desde una tarjeta de terminal, con sus conversaciones compartidas por todos los perfiles |
| Una carpeta en la que confiaste en tu cuenta principal se abre sin la pregunta "trust this folder?" en todos los demás perfiles | sí | sí | sin perfiles | no pregunta nada | no pregunta nada | no medido | sí | no pregunta nada |
| Una sesión en el sandbox se abre sin la pregunta "trust this folder?" | sí | sí | sí | no pregunta nada | no pregunta nada | sí, y ya no pregunta si inicializar el proyecto | sí | no pregunta nada |
| Un worktree de una carpeta en la que confiaste se abre sin la pregunta "trust this folder?" | sí | sí | sí | no pregunta nada | no pregunta nada | sí, y ya no pregunta si inicializar el proyecto | sí | no pregunta nada |
| Dirigir una sesión en ejecución desde `lich control` o un agente: prompt, detener, modelo, esfuerzo, slash command | sí | no | no | no | no | no | no | no |
| Un mensaje de otra sesión llega sin escribirse en la terminal | sí | no | no | no | no | no | no | no |
| Un subagente que inicia el agente se ejecuta como una sesión de lich con su propia tarjeta, en el checkout de la sesión que lo pidió, entrega su informe completo y se cierra al terminar; sus propios subagentes también reciben tarjeta, un nivel más abajo | sí | no | no | no | no | no | no | no |
| Hacer una pregunta lateral a una sesión en ejecución sin detenerla (`lich ask`) | sí | no | no | no | no | no | no | no |
| Contexto, costo y uso del plan tal como los midió el propio CLI | sí | no | no | no | no | no | no | no |
| La línea de estado de la terminal muestra a quién le debe una respuesta la sesión, de quién espera una y qué respuestas están listas para recoger | sí | no | no | no | no | no | no | no |
| Delegar en privado desde un subagente o un paso de workflow | sí | sí | sí | sí | sí | sí | sí | sí |
| Una sesión reanudada conserva el modelo y el esfuerzo que tenía | solo el modelo | sí | no | modelo | sí | no | modelo | modelo; esfuerzo sin probar |
| Un resultado te sigue esperando después de interrumpir la espera | sí | sí | sí | sí | no | sí | sí | sí |
| Una tarea que entregas a otra sesión informa "sin respuesta" en cuanto ese agente termina su turno sin responder | sí | sí | sí | sí | sí | no, espera al plazo | sí | sí |
| Los mensajes propios de lich al agente (tareas entregadas, resultados listos, avisos) en el idioma de prompt que elijas | sí | sí | sí | sí | sí | sí | sí | sí |
| Se le indica al iniciarse, en el idioma de prompt, que reparta el trabajo como sesiones de lich en vez de subagentes ocultos | sí | no | no | no | sí | no | no | no |

Cada carencia es deliberada, ninguna es lich ocultando algo que el CLI informa,
y [`docs/ceilings.md`](docs/ceilings.md) dice qué se midió detrás de cada una.

## Instalación

Una línea: detecta tu distro, verifica el checksum e instala el paquete nativo y
sus dependencias a través de tu gestor de paquetes:

```bash
curl -fsSL https://raw.githubusercontent.com/omartelo/lich/main/install.sh | sh
```

| Plataforma | Cómo obtenerlo | Necesita en ejecución |
| --- | --- | --- |
| **Linux** | el `install.sh` de arriba, o AUR [`lich-bin`](https://aur.archlinux.org/packages/lich-bin) (`yay -S lich-bin`) | `zenity`, además de glibc 2.34 o posterior para la ventana que viene en el paquete (Debian 12, Ubuntu 22.04, RHEL 9 y posteriores) |
| **macOS** | `brew install --cask lichdotdev/tap/lich` | nada, la ventana viene en la app tanto en Apple Silicon como en Intel |
| **Windows** | instalador o zip portable de [Releases](https://github.com/omartelo/lich/releases), o Scoop: `scoop install https://github.com/omartelo/lich/releases/latest/download/lich.json` | nada, la ventana viene con cada uno |

Paquetes manuales por distro, el tarball de Linux y el zip portable de Windows:
[INSTALL.md](INSTALL.md). Los binarios de macOS y Windows no están firmados:
Gatekeeper y SmartScreen avisan hasta que lleguen la notarización y la firma.
Las instalaciones con Homebrew evitan el aviso de Gatekeeper; una descarga desde
la página de Releases necesita que se le quite a mano la marca de cuarentena.
En macOS el cask instala `Lich.app`, así que lich tiene su propio icono en
`/Applications` y en el Dock mientras se ejecuta. Actualizar desde la fórmula
antigua requiere `brew uninstall lich` primero: [INSTALL.md](INSTALL.md) explica
por qué.

## Primeros pasos

1. **Instala** y abre `lich`.
2. **Abre un proyecto**: el `+` de la barra de pestañas lista lo que cerraste
   hace poco y abre el selector de carpetas de tu sistema operativo; apúntalo a
   un repositorio git.
3. **Indica a lich tu agente**: la primera vez que se abre lista los agentes que
   encontró en tu máquina; en Ajustes › Proveedores puedes fijar la ruta de cada
   binario y elegir el predeterminado. Un proyecto puede seguirlo o elegir un
   proveedor distinto en la misma pantalla.
4. **Inicia una sesión**: *Nueva sesión* lanza una terminal que ejecuta tu agente
   en el proyecto. El encabezado de cada checkout también tiene un menú `+` para
   abrir cualquier proveedor habilitado o una terminal simple en ese checkout
   exacto; haz clic en el propio encabezado para contraer o expandir sus
   sesiones.
5. **Ramifica un worktree** *(opcional)*: créalo desde cualquier rama base; lich
   lo prepara y te lleva a una sesión nueva.

## Configuración

- **Proveedores**: fija la ruta del binario de cada proveedor y el predeterminado
  para todos los proyectos en Ajustes › Proveedores. La fila del proyecto abierto
  puede reemplazar esa elección; **Limpiar** quita el reemplazo, así que los
  cambios posteriores al predeterminado se propagan solos. Sin ruta fijada, una
  sesión ejecuta el primero de los binarios del proveedor que haya en el `PATH`
  de tu shell de inicio de sesión; en la ventana nativa el bloque Binario nombra
  ese archivo y su versión, y lista cualquier otra versión que haya en el `PATH`,
  de lo que `lich doctor` también avisa. Para elegir una, fija su ruta en Binario
  (el ajuste `provider.<id>.bin`, p. ej. `provider.codex.bin`).
  La sección de un proveedor se abre
  con cuánto queda de tu plan, cuando lich puede leerlo. Lo que muestra el pie de
  ventana se define en Ajustes › Apariencia arrastrando sus elementos; el
  indicador de costo empieza oculto, ya que la cifra solo significa algo cuando
  te facturan por token.
- **Worktrees**: el `.lich/setup-worktree.sh` del checkout del proyecto se ejecuta
  en la terminal de un worktree nuevo antes del agente; el diálogo Nueva sesión
  en worktree lo muestra y ofrece una sugerencia detectada cuando el repositorio
  no trae ninguno. Un archivo `.worktreeinclude` ajusta qué archivos ignorados
  por git se copian.
- **Hooks de sesión**: con el
  [plugin de lich](https://github.com/lichdotdev/lich-plugin) instalado desde
  Ajustes, una sesión pone título a su propia tarjeta y actualiza git en el
  momento en que escribe un archivo.
- **Estabilidad**: qué partes de lich son seguras para automatizar con scripts y
  cuáles pueden cambiar en cualquier versión está escrito en
  [docs/stability.md](docs/stability.md).

## Privacidad y actualizaciones

Todo se ejecuta en tu máquina. Sin cuenta, sin inicio de sesión, sin telemetría:
el backend es un listener de loopback autenticado con token. Por sí solo, lich
sale a la red por esto y nada más:

- **Actualizaciones**: una comprobación de versión contra GitHub Releases al
  iniciar y cada hora, y la lista de versiones del plugin de lich en GitHub al
  iniciar.
- **Uso del plan**: el endpoint de uso de cada proveedor (Anthropic, OpenAI,
  Cursor, OpenCode Go), con el inicio de sesión que el CLI de la sesión ya usa,
  como máximo cada cinco minutos. El plan de Antigravity se lee del propio `agy`.
- **Precios**: la tabla pública de precios de LiteLLM en GitHub, una sola vez por
  cada modelo que la tabla incluida no conoce.

La compilación del instalador de Windows se actualiza en el lugar; una
instalación con Homebrew, Scoop, AUR o un paquete de Linux se actualiza a través
de su gestor de paquetes, y lich te da el comando.
Ajustes › Ayuda dice qué lleva el archivo de log (rutas, nombres de proyectos y
de ramas, tu login de gh, nunca un token de sesión) antes de que lo adjuntes a un
reporte de bug, y `lich rage` reúne ese reporte en un solo archivo sin subir
nada.

## Compilar desde el código fuente

Backend en Go puro (Go 1.27, `CGO_ENABLED=0`) que sirve un frontend embebido en
React 18 / TypeScript / Vite sobre un listener de loopback autenticado con token
(RPC HTTP + WebSockets). Las terminales son xterm.js con el addon WebGL; las
superficies de código y diff son CodeMirror 6. El shell de Chromium es un
registro de decisión: [`docs/chromium-shell.md`](docs/chromium-shell.md). Los
requisitos son **Go 1.27.0+**, **Node + pnpm** y **[Task](https://taskfile.dev)**;
no hace falta toolchain de C para el binario de Go. La ventana (`shell/`, Rust
sobre CEF) añade un toolchain de Rust, además de las bibliotecas de desarrollo de
Chromium en Linux y MSVC en Windows;
[CONTRIBUTING.md](CONTRIBUTING.md) las lista.

```bash
task dev      # hot-reload dev mode (Vite on :9245)
task build    # production binary -> bin/lich
task run      # build + run
task test     # Go + frontend suites
```

Empaqueta una versión de Linux en local (requiere
[nfpm](https://nfpm.goreleaser.com/)):

```bash
task package   # .deb + .rpm + Arch .pkg.tar.zst in bin/
```

Añadir otro CLI de agente a los ocho que ejecuta lich es el cambio que llega a
una docena de archivos en dos repositorios:
[`docs/adding-a-provider.md`](docs/adding-a-provider.md) es el mapa.

## Patrocinadores

lich lo escribe y lo mantiene una sola persona. Patrocinar paga el tiempo que se
le dedica y mantiene independiente al proyecto: no hay un nivel de pago de la app
y no lo habrá.

[**Hazte patrocinador**](https://github.com/sponsors/omartelo)

<!-- sponsor-names: monthly sponsors go here -->

### Colaboradores

<!-- backers: one-time supporters go here -->

Todavía nadie.

## Licencia

[AGPL-3.0-only](LICENSE) © 2026 omartelo

lich es software libre: puedes usarlo, estudiarlo, modificarlo y redistribuirlo
bajo los términos de la Licencia Pública General de Affero de GNU v3. Todo
derivado distribuido o servido en red debe publicarse bajo la misma licencia. Las
versiones hasta la v0.9.0 incluida siguen bajo licencia MIT.
