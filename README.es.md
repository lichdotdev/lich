<div align="center">
  <img src="docs/media/appicon.svg" alt="lich" width="88" height="88" />
  <h1>lich</h1>
  <p><a href="README.md">English</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <strong>Español</strong> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.ru.md">Русский</a></p>
  <p><strong>Un ADE de terminal para los agentes de código que ya usas.</strong></p>
  <p>
    Abre tus proyectos, corre agentes como Claude Code, Codex y opencode en
    terminales de verdad y ten git (worktrees, diffs y pull requests) a la vista
    sin salir de la ventana. Sin Electron y sin navegador: lich dibuja su propia
    ventana nativa con Gio, en Linux, Windows y macOS.
  </p>
  <p><a href="https://lichdotdev.github.io/lich/"><strong>lichdotdev.github.io/lich</strong></a></p>
  <p>
    <a href="https://github.com/lichdotdev/lich/releases"><img alt="Release" src="https://img.shields.io/github/v/release/lichdotdev/lich?color=4285F4&label=release" /></a>
    <img alt="Go" src="https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white" />
    <img alt="Window" src="https://img.shields.io/badge/window-native%20(Gio)-4285F4" />
    <img alt="Platform" src="https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-333" />
    <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-AGPL--3.0-blue" /></a>
    <a href="https://github.com/sponsors/omartelo"><img alt="Sponsor" src="https://img.shields.io/github/sponsors/omartelo?color=ea4aaa&logo=githubsponsors&label=sponsors" /></a>
  </p>
  <img src="docs/media/session.png" alt="Cuatro sesiones de Claude Code una al lado de la otra en un mismo grupo, cada una en su propio git worktree; la barra lateral muestra cada una con su rama y su indicador de diff, y en el pie de ventana se ven el modelo, el plan y la rama" width="1000" />
  <!-- sponsor-logos: company logos go here, between the screenshot and Why lich -->
</div>

## Por qué lich

lich te permite:

- **Usar el agente que ya tienes.** [Claude Code](https://www.anthropic.com/claude-code),
  [Codex](https://github.com/openai/codex), Antigravity,
  [opencode](https://github.com/sst/opencode), oh-my-pi,
  [Crush](https://github.com/charmbracelet/crush), el
  [Cursor CLI](https://cursor.com/docs/cli) y el
  [Kiro CLI](https://kiro.dev/docs/cli/) funcionan aquí todos igual. Le
  indicas a lich una sola vez dónde está cada binario y después eliges el
  predeterminado o uno distinto por sesión. Lo que lich consigue *leer* de una
  sesión depende del proveedor; el detalle está en la tabla de
  [Compatibilidad por proveedor](#compatibilidad-por-proveedor).
- **Tener una terminal de verdad.** Shells sobre PTY, varios por proyecto,
  renderizados en la GPU, con un historial en el que puedes buscar y que
  sobrevive a cerrar y volver a abrir la ventana. Si a una le pones un punto de
  entrada (`lazygit`, `k9s`, `pnpm dev`), se abre siempre directo en esa
  herramienta, igual en Linux, macOS y Windows: no se carga antes ningún rc del
  shell ni perfil de PowerShell, así que el punto de entrada que te funciona a
  ti también le funciona a cualquiera con quien lo compartas. El pie de ventana
  sigue cada `cd` y muestra la rama. En una sesión de Claude Code, Codex o Kiro
  CLI muestra además el modelo y la ventana de contexto en uso; con Claude Code,
  Codex, Antigravity, Cursor CLI y una suscripción de OpenCode Go, cuánto te
  queda de la ventana móvil del plan; y, si lo pides, cuánto lleva gastado la
  sesión, en los proveedores que lo registran.
- **Poner una sesión a trabajar para otra.** Le pasas una tarea a otra tarjeta y
  el agente de esa tarjeta te devuelve la respuesta, sea cual sea el agente de
  cada lado: llega a las demás sesiones con herramientas que recibe al
  arrancar (MCP en Claude Code y Codex) o que trae el plugin. Todo eso también
  está disponible como el comando `lich` en cualquier shell, con `--json`
  incluido, así que un script puede controlar una sesión sin ningún agente de
  por medio ([`docs/cli.md`](docs/cli.md)).
- **Seguir varias a la vez.** Pon cualquier sesión en un grupo junto a la que
  estás usando y lich acomoda los paneles por su cuenta, según cuántos sean y el
  espacio que tenga la ventana: ocho sesiones quedan en filas de cuatro en un
  monitor ultrawide y de dos en un portátil. Una sesión que delegó trabajo en
  otras arma el grupo a su alrededor con un clic. Cada grupo tiene nombre, un
  proyecto puede tener todos los que quieras, y cada uno aparece como un bloque
  en la barra lateral que puedes plegar, renombrar o deshacer.
- **Crear un worktree sin preparar nada a mano.** Créalo a partir de cualquier
  rama base y lich le copia tus archivos `.env*` ignorados por git y le asigna un
  puerto para el servidor de desarrollo que no usa ningún otro checkout ni
  ningún proceso de la máquina. Si partes de los cambios sin commitear del
  checkout, se vienen con él, incluidos los archivos sin seguimiento.
- **Correr una sesión aislada.** Un agente puede abrirse dentro de un sandbox del
  sistema operativo: un home nuevo y vacío, solo con el estado de ese proveedor,
  el resto de la máquina en solo lectura y permiso de escritura únicamente en el
  checkout para el que se abrió. Las claves ssh, las credenciales de la nube y
  los demás repositorios del disco simplemente no están ahí, y eso compensa
  dejar que el agente se salte las confirmaciones de permisos. En Linux usa
  bubblewrap y en macOS `sandbox-exec`; no protege frente a código malicioso, y
  [`docs/ceilings.md`](docs/ceilings.md) explica qué es lo que no frena.
- **Revisar el diff justo donde lo lees.** Un panel de diff muestra los
  cambios del working tree junto a un árbol de archivos en vivo, pliega si se lo
  pides los que solo tocan espacios en blanco, y muestra el antes y el después de
  una imagen o un PDF modificado. Haz clic derecho sobre una selección para
  comentar esas líneas; todo el lote se pega en la sesión como un solo prompt,
  sin enviarlo. Su pestaña Código permite navegar el checkout y buscar en el
  texto de sus archivos.
- **Llevar el pull request hasta el final sin salir de aquí.** Ve los pull
  requests abiertos del repositorio, haz checkout de uno en su propio worktree,
  lee el diff, revísalo inline y fusiónalo con los métodos que la rama base
  acepta de verdad. En una rama que todavía no tiene pull request, **Crear con el
  agente** le pide a la sesión de esa rama que lo abra, y no se envía nada hasta
  que presiones Enter.

Y además: [temas](docs/themes.md) que importas como JSON o instalas desde un
repositorio git, una paleta `Ctrl`/`Cmd`+`K` que te lleva a una sesión por su
nombre o por algo que se dijo en la conversación, una notificación de escritorio
cuando una sesión te está esperando, y `lich rage` / `lich doctor` para cuando
directamente no arranca.

El proyecto está en desarrollo activo: los bugs y los pedidos de funciones van a
[Issues](https://github.com/lichdotdev/lich/issues), y lo que cambió en cada
versión está en [CHANGELOG.md](CHANGELOG.md).

## Compatibilidad por proveedor

Todos los proveedores arrancan en una terminal de verdad, se reanudan por id de
conversación, se aíslan con el mismo sandbox y reciben la misma forma de llegar
a tus otras sesiones. Lo que cambia es lo que lich puede *leer* de cada sesión,
porque lee lo que cada CLI deja escrito, y no hay dos que registren lo mismo.

| Lo que obtienes | Claude Code | Codex | Antigravity | opencode | oh-my-pi | Crush | Cursor CLI | Kiro CLI |
| --- | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: |
| Ventana de contexto en el pie de ventana | sí | sí | no | no | no | no | no | sí |
| Cuánto del último turno salió de la caché de prompts, en el pie de ventana | sí | sí | no | en el tooltip del costo | en el tooltip del costo | no | no | no |
| El tooltip de la tarjeta nombra el modelo en uso, y con cuál se abrió si cambió | sí | sí | no | no | no | no | no | sí |
| Costo en el pie de ventana | sí | sí | no | sí | sí | sí | no | créditos |
| Cuánto queda de tu plan | sí | sí | sí | solo OpenCode Go | no | no | sí | no |
| Un turno que el límite de uso de tu plan detuvo se reanuda cuando el límite se restablece | sí | sí | no | sus propios reintentos | sus propios reintentos | no | no | no |
| Spinner mientras corre un turno, anillo cuando termina | sí | sí | sí | sí | sí | no | sí | sí |
| La tarjeta toma el nombre de la conversación | sí | por tu primer prompt | por tu primer prompt | sí | sí, desde el segundo turno | no | sí, cuando termina el primer turno | por tu primer prompt, cuando termina el primer turno |
| Un pull request que abre el agente aparece en su tarjeta al terminar el turno | sí | sí | sí | sí | sí | no, cuando la ventana recupera el foco | sí | sí |
| Se repinta con el aspecto nuevo al cambiar lich entre un tema claro y uno oscuro | sí | no | no | sí | sí | no | sí | no |
| Campana cuando el agente está bloqueado esperándote | sí | sí | no | sí | no | no | no | no |
| Ctrl+Shift+Y salta a la siguiente sesión que te espera | sí | sí | no | sí | no | no | no | no |
| La tarjeta avisa cuando la conversación se está compactando | sí | no | no | no | no | no | no | no |
| Filtro de la barra lateral por estado: En espera, En ejecución, Sin leer | sí | sí | no En espera | sí | no En espera | no, siempre Inactiva | no En espera | no En espera |
| Si tu agente lo ejecuta dentro de otra sesión (`claude -p`, `cursor-agent -p`), no aparece en la tarjeta de esa sesión | sí | sí | sí | sí | sí | sí | sí | no |
| La máquina no entra en reposo mientras corre un turno | sí | sí | sí | sí | sí | no | sí | sí |
| **Último turno** de la pestaña Revisión, con el resumen del agente | sí | sí | sí | sí | sí | no | sí | sí |
| Buscar en la conversación desde la paleta | sí | sí | sí | sí | sí | sí | no | sí |
| Reanudar una conversación iniciada fuera de lich, desde la pestaña Historial de la paleta | sí | sí | sí | sí | sí | Linux; macOS y Windows sin probar | sí | sí |
| Un pegado largo puede llegar como texto editable en vez de un marcador plegado (Ajustes › Proveedores, desactivado por defecto) | sí | no | no | sí | no medido | no, se convierte en un adjunto `paste_N.txt` | no | sí |
| Bifurcar una conversación en un nuevo worktree, o en su propio checkout | sí | sí | no | sí | no | no | no | no |
| Aviso cuando retomas, desde la propia app del agente, una conversación que otra tarjeta tiene abierta | sí | en tu próximo prompt | en tu próximo prompt, sin probar | no | sí | en su próxima llamada a una herramienta | no | sí |
| Abrir una sesión con un esfuerzo de razonamiento elegido | sí | sí | sí | no | sí | no | en el nombre del modelo | sí |
| Abrir una sesión con ultracode activado | sí | no | no | no | no | no | no | no |
| Varias cuentas lado a lado como perfiles, cada una con su propio inicio de sesión completo, y una sesión movida a otra cuenta que sigue en la misma tarjeta (Ajustes › Proveedores) | sí | sí, con sus propios plugins en cada perfil y los ajustes de Personal compartidos por todos | no | sí, con sesión iniciada desde una tarjeta de terminal; una sesión no puede pasar a otro perfil, porque sus conversaciones están en la base de datos que guarda el inicio de sesión | sí, con sesión iniciada desde una tarjeta de terminal, con sus ajustes compartidos por todos los perfiles | no medido | Linux y Windows (en macOS su inicio de sesión está en el Llavero), con sesión iniciada desde una tarjeta de terminal, con `mcp.json`, hooks y skills compartidos por todos los perfiles | solo Linux (macOS guarda su inicio de sesión en el Llavero y en Windows nada lo mueve), con sesión iniciada desde una tarjeta de terminal, con sus conversaciones compartidas por todos los perfiles |
| Una carpeta en la que confiaste en tu cuenta principal se abre sin la pregunta "trust this folder?" en todos los demás perfiles | sí | sí | sin perfiles | no pregunta nada | no pregunta nada | no medido | sí | no pregunta nada |
| Una sesión en sandbox se abre sin la pregunta "trust this folder?" | sí | sí | sí | no pregunta nada | no pregunta nada | sí, y ya no pregunta si inicializar el proyecto | sí | no pregunta nada |
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

Ninguna de estas diferencias es un descuido, y en ningún caso lich se guarda
algo que el CLI informe; [`docs/ceilings.md`](docs/ceilings.md) cuenta qué se
midió detrás de cada una.

## Instalación

Con una sola línea: el script detecta tu distro, verifica el checksum e instala
el paquete nativo y sus dependencias con tu gestor de paquetes:

```bash
curl -fsSL https://raw.githubusercontent.com/lichdotdev/lich/main/install.sh | sh
```

| Plataforma | Cómo obtenerlo | Necesita en ejecución |
| --- | --- | --- |
| **Linux** | el `install.sh` de arriba, o AUR [`lich-bin`](https://aur.archlinux.org/packages/lich-bin) (`yay -S lich-bin`) | `zenity`, además de glibc 2.34 o posterior para la ventana que viene en el paquete (Debian 12, Ubuntu 22.04, RHEL 9 y posteriores) |
| **macOS** | `brew install --cask lichdotdev/tap/lich` | nada, la ventana viene en la app tanto en Apple Silicon como en Intel |
| **Windows** | instalador o zip portable de [Releases](https://github.com/lichdotdev/lich/releases), o Scoop: `scoop install https://github.com/lichdotdev/lich/releases/latest/download/lich.json` | nada, la ventana viene con cada uno |

Paquetes manuales por distro, el tarball de Linux y el zip portable de Windows:
[INSTALL.md](INSTALL.md). Los binarios de macOS y Windows no están firmados:
Gatekeeper y SmartScreen avisan hasta que lleguen la notarización y la firma.
Si instalas con Homebrew no ves el aviso de Gatekeeper; si descargas desde la
página de Releases, tienes que quitarle a mano la marca de cuarentena.
En macOS el cask instala `Lich.app`, así que lich tiene su propio icono en
`/Applications` y en el Dock mientras se ejecuta. Actualizar desde la fórmula
antigua requiere `brew uninstall lich` primero: [INSTALL.md](INSTALL.md) explica
por qué.

## Primeros pasos

1. **Instala** y abre `lich`.
2. **Abre un proyecto**: el `+` de la barra de pestañas lista lo que cerraste
   hace poco y abre el selector de carpetas de tu sistema operativo; apúntalo a
   un repositorio git.
3. **Indícale a lich tu agente**: la primera vez que lo abres, lista los agentes
   que encontró en tu máquina; en Ajustes › Proveedores puedes definir la ruta de
   cada binario y elegir el predeterminado. En esa misma pantalla, cada proyecto
   puede usar ese predeterminado o elegir otro proveedor.
4. **Inicia una sesión**: *Nueva sesión* abre una terminal con tu agente
   corriendo en el proyecto. El encabezado de cada checkout también tiene un menú
   `+` para abrir cualquier proveedor habilitado, o una terminal sin agente,
   justo en ese checkout; haz clic en el encabezado para contraer o expandir sus
   sesiones.
5. **Crea un worktree** *(opcional)*: créalo a partir de cualquier rama base;
   lich lo prepara y te deja en una sesión nueva.

## Configuración

- **Proveedores**: en Ajustes › Proveedores defines la ruta del binario de cada
  proveedor y el predeterminado para todos los proyectos. La fila del proyecto
  abierto puede sobrescribir esa elección; **Borrar** quita esa excepción, así
  que los cambios que hagas después en el predeterminado le llegan solos. Si no
  defines ninguna ruta, la sesión ejecuta el primer binario del proveedor que
  encuentre en el `PATH` de tu login shell; el bloque
  Binario muestra qué archivo es y su versión, y lista cualquier otra versión que
  haya en el `PATH`, algo de lo que `lich doctor` también avisa. Para quedarte
  con una, pon su ruta en Binario (el ajuste `provider.<id>.bin`, p. ej.
  `provider.codex.bin`).
  La sección de cada proveedor empieza mostrando
  cuánto te queda del plan, cuando lich puede leerlo. Lo que aparece en el pie de
  ventana se elige en Ajustes › Apariencia arrastrando sus elementos; el costo
  arranca oculto, porque esa cifra solo tiene sentido si pagas por token.
- **Worktrees**: con un archivo `.worktreeinclude` en el checkout del proyecto
  eliges qué archivos ignorados por git se copian a cada worktree nuevo.
- **Hooks de sesión**: con el
  [plugin de lich](https://github.com/lichdotdev/lich-plugin) instalado desde
  Ajustes, cada sesión le pone título a su propia tarjeta y refresca el estado
  de git en cuanto escribe un archivo.
- **Estabilidad**: en [docs/stability.md](docs/stability.md) está escrito en qué
  partes de lich puedes apoyarte desde un script y cuáles pueden cambiar en
  cualquier versión.

## Privacidad y actualizaciones

Todo corre en tu máquina. Sin cuenta, sin login, sin telemetría: el backend es un
listener en loopback autenticado con token. Por iniciativa propia, lich solo se
conecta a la red para esto:

- **Actualizaciones**: consulta la última versión en GitHub Releases al arrancar
  y cada hora, y la lista de versiones del plugin de lich en GitHub al arrancar.
- **Uso del plan**: el endpoint de uso de cada proveedor (Anthropic, OpenAI,
  Cursor, OpenCode Go), con el mismo login que ya usa el CLI de la sesión, como
  mucho cada cinco minutos. El plan de Antigravity se lee del propio `agy`.
- **Precios**: la tabla pública de precios de LiteLLM en GitHub, una sola vez por
  cada modelo que no esté en la tabla que trae lich.

La versión del instalador de Windows se actualiza sola; si instalaste con
Homebrew, Scoop, AUR o un paquete de Linux, la actualización pasa por tu gestor
de paquetes y lich te da el comando.
Antes de que adjuntes el archivo de log a un reporte de bug, Ajustes › Ayuda te
dice qué contiene (rutas, nombres de proyectos y de ramas, tu login de gh, nunca
un token de sesión), y `lich rage` junta ese reporte en un solo archivo sin
subir nada a ningún lado.

## Compilar desde el código fuente

lich son dos binarios. El backend, `lich`, es Go puro (Go 1.27, `CGO_ENABLED=0`,
un único binario estático) y sirve las sesiones a través de un listener en
loopback autenticado con token (RPC por HTTP + WebSockets). La ventana,
`lich-native`, es un módulo de Go propio en `native/`, compilado con cgo:
[Gio](https://gioui.org) la dibuja, `libghostty-vt` ejecuta sus terminales y
[godemirror](https://github.com/lichdotdev/godemirror) sus vistas de código y de
diff. Necesitas **Go 1.27.0+** y **[Task](https://taskfile.dev)**; la ventana
suma un toolchain de C, los paquetes de sistema de Gio y una `libghostty-vt`
compilada con Zig 0.16 desde el commit de Ghostty en `native/GHOSTTY_COMMIT`.
[`native/slice.sh`](native/slice.sh) explica cómo compilarla, y
[`docs/native-window.md`](docs/native-window.md) describe la ventana.

```bash
task build          # binario del backend -> bin/lich
task build:native   # ventana -> bin/lich-native
task run            # compila + ejecuta
task test           # suites de Go
```

Para empaquetar una versión de Linux en local (requiere
[nfpm](https://nfpm.goreleaser.com/)):

```bash
task package   # .deb + .rpm + Arch .pkg.tar.zst in bin/
```

Sumar otro CLI de agente a los ocho que ya corre lich es un cambio que toca una
docena de archivos repartidos en dos repositorios; el mapa está en
[`docs/adding-a-provider.md`](docs/adding-a-provider.md).

## Patrocinadores

lich lo escribe y lo mantiene una sola persona. El patrocinio paga el tiempo
que se le dedica y mantiene el proyecto independiente: la app no tiene una
versión de pago y no la va a tener.

[**Hazte patrocinador**](https://github.com/sponsors/omartelo)

<!-- sponsor-names: monthly sponsors go here -->

### Apoyos puntuales

<!-- backers: one-time supporters go here -->

Todavía nadie.

## Licencia

[AGPL-3.0-only](LICENSE) © 2026 omartelo

lich es software libre: puedes usarlo, estudiarlo, modificarlo y redistribuirlo
bajo los términos de la GNU Affero General Public License v3. Cualquier obra
derivada que se distribuya o se ofrezca a través de la red debe publicarse bajo
la misma licencia. Las versiones hasta la v0.9.0, inclusive, siguen bajo
licencia MIT.
