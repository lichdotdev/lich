<div align="center">
  <img src="docs/media/appicon.svg" alt="lich" width="88" height="88" />
  <h1>lich</h1>
  <p><a href="README.md">English</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <strong>Español</strong> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.ru.md">Русский</a></p>
  <p>
    <a href="https://github.com/lichdotdev/lich/releases"><img alt="Release" src="https://img.shields.io/github/v/release/lichdotdev/lich?color=4285F4&label=release" /></a>
    <img alt="Platform" src="https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-333" />
    <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-AGPL--3.0-blue" /></a>
    <a href="https://github.com/sponsors/omartelo"><img alt="Sponsor" src="https://img.shields.io/github/sponsors/omartelo?color=ea4aaa&logo=githubsponsors&label=sponsors" /></a>
  </p>
  <p>
    <strong>Tus agentes de código lado a lado, cada uno en su propia rama.</strong><br/>
    Claude Code, Codex, opencode o el agente que ya usas, cada uno en su propio worktree, todos en una sola ventana.
  </p>
  <h3><a href="#instalación">Descargar lich</a> · <a href="https://lichdotdev.github.io/lich/index.es.html">lichdotdev.github.io/lich</a></h3>
  <img src="docs/media/session.png" alt="Cuatro sesiones de Claude Code una al lado de la otra en un mismo grupo, cada una en su propio git worktree. La barra lateral las lista con su rama y su indicador de diff, con los servicios en marcha del proyecto y el perfil al pie, y el pie de ventana muestra la rama, el modelo y el plan" width="1000" />
  <!-- sponsor-logos: company logos go here, between the screenshot and Features -->
</div>

## Funciones

<table>
<tr>
<td width="50%" valign="middle">

### Agentes lado a lado

Pon las sesiones en un grupo y lich acomoda los paneles por ti. Mira de un vistazo cuál está ocupada, cuál te está esperando y cuánto contexto le queda.

</td>
<td width="50%"><img src="docs/media/wall-mixed.png" alt="Sesiones de Claude Code y Codex en un mismo grupo, cada una en su propia rama" width="100%" /></td>
</tr>
<tr>
<td width="50%" valign="middle">

### Un worktree por tarea, sin preparar nada

Crea una sesión a partir de cualquier rama base y lich le copia tus archivos `.env` y le asigna un puerto libre para el servidor de desarrollo. Tus cambios sin commitear pueden venir con ella.

</td>
<td width="50%"><img src="docs/media/worktrees.png" alt="Sesiones en la barra lateral, cada una con su propia rama y su indicador de diff" width="100%" /></td>
</tr>
<tr>
<td width="50%" valign="middle">

### Revisa el diff, comenta las líneas

Lee lo que cambió el agente junto a su sesión, comenta las líneas que necesitan retoque y devuelve todas las notas como un solo prompt.

</td>
<td width="50%"><img src="docs/media/review-comments.png" alt="Un diff con dos comentarios de línea en cola para enviar a la sesión" width="100%" /></td>
</tr>
<tr>
<td width="50%" valign="middle">

### Señala la página, díselo al agente

El agente maneja un navegador en una pestaña junto a su sesión, y tú usas la misma página. Señala lo que está mal, deja una nota encima y envía todas las notas de vuelta en un solo prompt.

</td>
<td width="50%"><img src="docs/media/page-notes.png" alt="El navegador de la sesión junto a su terminal, con dos notas numeradas fijadas a elementos de la página" width="100%" /></td>
</tr>
<tr>
<td width="50%" valign="middle">

### Pull requests sin el navegador

Abre un pull request, pídele al agente de la rama que lo escriba, revísalo inline y fusiónalo, todo desde la misma ventana.

</td>
<td width="50%"><img src="docs/media/pull-request-review.png" alt="Un pull request abierto en lich con su árbol de archivos, el diff y un comentario de revisión pendiente" width="100%" /></td>
</tr>
<tr>
<td width="50%" valign="middle">

### Hazlo tuyo

Importa un tema como JSON o instala uno desde un repositorio git.

[Docs →](docs/themes.md)

</td>
<td width="50%"><img src="docs/media/themes.png" alt="Los ajustes de Apariencia con el selector de tema, las fuentes y la disposición del pie de ventana" width="100%" /></td>
</tr>
</table>

Y además:

- **Sesiones que delegan.** Un agente le pasa una tarea a otra sesión y recibe la respuesta. Lo mismo funciona desde tu shell con el comando `lich` ([docs](docs/cli.md)).
- **Agentes en sandbox.** Deja que un agente se salte las confirmaciones de permisos dentro de un sandbox que no ve tus claves, tus credenciales ni tus otros repositorios ([límites](docs/ceilings.md)).
- **Tus servidores de desarrollo en la barra lateral.** Guarda lo que un proyecto mantiene en marcha y arráncalo o detenlo con un clic.
- **Encuentra lo que sea.** `Ctrl`/`Cmd`+`K` te lleva a una sesión por su nombre o por algo que se dijo en ella.
- **Entérate cuando te necesitan.** Una notificación de escritorio cuando una sesión te está esperando.

## Instalación

```bash
curl -fsSL https://raw.githubusercontent.com/lichdotdev/lich/main/install.sh | sh
```

Una sola línea: detecta tu distro, verifica la descarga e instala lich con tu gestor de paquetes.

| Plataforma | Cómo obtenerlo |
| --- | --- |
| **Linux** | el `install.sh` de arriba, o AUR [`lich-bin`](https://aur.archlinux.org/packages/lich-bin) (`yay -S lich-bin`). Necesita `zenity` y glibc 2.34 o posterior (Debian 12, Ubuntu 22.04, RHEL 9 y posteriores). |
| **macOS** | `brew install --cask lichdotdev/tap/lich` |
| **Windows** | instalador o zip portable de [Releases](https://github.com/lichdotdev/lich/releases), o `scoop install https://github.com/lichdotdev/lich/releases/latest/download/lich.json` |

Los binarios de macOS y Windows todavía no están firmados, así que Gatekeeper y
SmartScreen avisan al descargar desde Releases; Homebrew evita el aviso de
Gatekeeper. Los paquetes manuales por distro, el tarball de Linux, cómo quitar la
marca de cuarentena y cómo actualizar desde la fórmula antigua de Homebrew están
en [INSTALL.md](INSTALL.md).

## Primeros pasos

1. **Abre un proyecto.** El `+` de la barra de pestañas lista lo que cerraste
   hace poco y abre un selector de carpetas. Apúntalo a un repositorio git.
2. **Elige tu agente.** La primera vez que lo abres, lista los agentes que
   encontró en tu máquina. En Ajustes › Proveedores defines el predeterminado y
   la ruta de cada binario.
3. **Inicia una sesión.** *Nueva sesión* abre tu agente en el proyecto. El `+` del
   encabezado de un checkout abre cualquier agente, o una terminal sin agente,
   en ese checkout.
4. **Dale un worktree** *(opcional)*. Créalo a partir de cualquier rama base y
   lich abre una sesión nueva en él.

## Compatibilidad por proveedor

Todos los agentes corren en una terminal de verdad, reanudan su conversación,
funcionan en el sandbox y llegan a tus otras sesiones. Lo que cambia de uno a
otro es lo que lich puede *leer* de él, porque cada CLI deja escrito algo
distinto.

<details>
<summary>Ver qué admite cada agente</summary>

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
| La tarjeta de la sesión abre un navegador que el agente conduce, en una pestaña junto a la sesión | sí | sí | sí | sí | sí | sí | sí | sí |

</details>

Ninguna de estas diferencias es un descuido, y [`docs/ceilings.md`](docs/ceilings.md)
cuenta qué se midió detrás de cada una.

## Configuración

- **Proveedores.** En Ajustes › Proveedores defines la ruta del binario de cada
  agente y el predeterminado para todos los proyectos; un proyecto puede elegir
  el suyo. Si no defines ninguna ruta, lich ejecuta el primer binario que
  encuentre en el `PATH` de tu login shell, y `lich doctor` avisa cuando hay más
  de una versión.
- **Pie de ventana.** En Ajustes › Apariencia, arrastra sus elementos para elegir
  qué muestra. El costo arranca oculto, porque esa cifra solo tiene sentido si
  pagas por token.
- **Worktrees.** Con un archivo `.worktreeinclude` en el proyecto eliges qué
  archivos ignorados por git se copian a cada worktree nuevo.
- **Hooks de sesión.** Instala el
  [plugin de lich](https://github.com/lichdotdev/lich-plugin) desde Ajustes y
  cada sesión le pone título a su propia tarjeta y refresca el estado de git en
  cuanto escribe un archivo.
- **Scripts.** En [docs/stability.md](docs/stability.md) está escrito en qué
  partes de lich puedes apoyarte desde un script.

## Privacidad y actualizaciones

Todo corre en tu máquina: sin cuenta, sin login, sin telemetría. lich solo se
conecta a la red para esto:

- **Actualizaciones**: consulta la última versión en GitHub Releases al arrancar
  y cada hora, y la lista de versiones del plugin de lich al arrancar.
- **Uso del plan**: el endpoint de uso de cada proveedor (Anthropic, OpenAI,
  Cursor, OpenCode Go), con el mismo login que ya usa el agente, como mucho cada
  cinco minutos. El plan de Antigravity se lee del propio `agy`.
- **Precios**: la tabla pública de precios de LiteLLM en GitHub, una sola vez por
  cada modelo que no esté en la tabla que trae lich.

El instalador de Windows se actualiza en el lugar; Homebrew, Scoop, AUR y los
paquetes de Linux se actualizan con su gestor de paquetes, y lich te da el
comando. ¿Algo no funciona? `lich doctor` revisa tu instalación y `lich rage`
junta un reporte de bug en un solo archivo sin subir nada. Antes de que adjuntes
el archivo de log, Ajustes › Ayuda te dice qué contiene (rutas, nombres de
proyectos y de ramas, tu login de gh, nunca un token de sesión). Los bugs y los
pedidos de funciones van a [Issues](https://github.com/lichdotdev/lich/issues);
lo que cambió en cada versión está en [CHANGELOG.md](CHANGELOG.md).

## Compilar desde el código fuente

lich son dos binarios: `lich`, un backend estático en Go puro, y `lich-native`,
la ventana, compilada con cgo desde `native/`. Necesitas **Go 1.27.0+** y
**[Task](https://taskfile.dev)**; la ventana suma un toolchain de C, los
paquetes de sistema de Gio y una `libghostty-vt` compilada desde el commit en
`native/GHOSTTY_COMMIT`. [`native/slice.sh`](native/slice.sh) explica cómo, y
[`docs/native-window.md`](docs/native-window.md) describe la ventana.

```bash
task build          # binario del backend -> bin/lich
task build:native   # ventana -> bin/lich-native
task run            # compila + ejecuta
task test           # suites de Go
task package        # .deb + .rpm + Arch .pkg.tar.zst en bin/ (requiere nfpm)
```

Sumar otro agente toca una docena de archivos repartidos en dos repositorios:
[`docs/adding-a-provider.md`](docs/adding-a-provider.md) es el mapa.

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
