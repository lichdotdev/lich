<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="frontend/public/appicon.png" />
    <img src="frontend/public/appicon-light.png" alt="lich" width="88" height="88" />
  </picture>
  <h1>lich</h1>
  <p><a href="README.md">English</a> · <strong>Português (Brasil)</strong> · <a href="README.es.md">Español</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.ru.md">Русский</a></p>
  <p><strong>Um ADE que prioriza o terminal, para os agentes de código que você já usa.</strong></p>
  <p>
    Abra seus projetos, rode agentes como Claude Code, Codex e opencode em
    terminais de verdade e mantenha o git (worktrees, diffs e pull requests) à
    vista sem sair da janela. Um único binário Go estático, sem Electron: a
    interface abre no Chromium embutido do próprio lich no Linux, no Windows e
    no macOS.
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
  <img src="docs/media/session.png" alt="Quatro sessões do Claude Code lado a lado em um mesmo split, cada uma em sua própria git worktree: a barra lateral as lista com a branch e o selo de diff, e o rodapé mostra o modelo, o plano e a branch" width="1000" />
  <!-- sponsor-logos: company logos go here, between the screenshot and Why lich -->
</div>

## Por que o lich

O lich permite que você:

- **Rode o agente que você já tem.** [Claude Code](https://www.anthropic.com/claude-code),
  [Codex](https://github.com/openai/codex), Antigravity,
  [opencode](https://github.com/sst/opencode), oh-my-pi,
  [Crush](https://github.com/charmbracelet/crush), o
  [Cursor CLI](https://cursor.com/docs/cli) e o
  [Kiro CLI](https://kiro.dev/docs/cli/) rodam aqui do mesmo jeito. Aponte o
  lich para cada binário uma única vez e depois escolha o padrão ou um por
  sessão. O que o lich consegue ler *de volta* de uma sessão varia conforme o
  provedor, e a tabela em [Suporte por provedor](#suporte-por-provedor) mostra
  o quê.
- **Tenha um terminal de verdade.** Shells com PTY, vários por projeto,
  renderizados na GPU, com scrollback pesquisável que sobrevive a um reload
  completo da página. Dê a um deles um entrypoint (`lazygit`, `k9s`,
  `pnpm dev`) e ele abre direto nessa ferramenta toda vez, no Linux, no macOS
  e no Windows: nenhum rc do shell nem perfil do PowerShell é carregado antes,
  então um entrypoint que funciona para você funciona para quem você
  compartilhar. O rodapé acompanha o `cd` e mostra a branch; para uma sessão
  do Claude Code, do Codex ou do Kiro CLI, também o modelo e a janela de
  contexto em uso; para Claude Code, Codex, Antigravity, Cursor CLI e uma
  assinatura do OpenCode Go, quanto da janela móvel do seu plano ainda resta;
  e, se você pedir, quanto a sessão gastou, nos provedores que registram isso.
- **Coloque uma sessão para trabalhar para outra.** Entregue uma tarefa a
  outro card e o agente dele escreve a resposta de volta, seja o que for que
  rode em cada ponta: o agente alcança as outras sessões por ferramentas
  entregues na criação (MCP para Claude Code e Codex) ou trazidas pelo plugin.
  Toda essa superfície também funciona como o comando `lich` em qualquer
  shell, `--json` incluído, então um script pode conduzir uma sessão sem
  nenhum agente no meio ([`docs/cli.md`](docs/cli.md)).
- **Acompanhe várias ao mesmo tempo.** Ponha qualquer sessão em um split ao
  lado da que você está usando e o lich organiza os painéis sozinho, conforme
  quantos são e quanto espaço a janela tem: oito ficam quatro por linha num
  monitor ultrawide e duas por linha num notebook. Uma sessão que delegou
  trabalho a outras monta o split ao redor de si com um clique. Cada split tem
  nome, um projeto guarda quantos você quiser, e cada um ganha um bloco na
  barra lateral que você pode recolher, renomear ou desfazer.
- **Crie uma branch em uma worktree sem o trabalho de preparar.** Abra uma a
  partir de qualquer branch base e o lich a popula com seus arquivos `.env*`
  ignorados pelo git, entrega uma porta de servidor de desenvolvimento que
  nenhum outro checkout nem processo da máquina esteja usando e roda o script
  de setup do projeto antes de o agente começar. Se você a iniciar a partir
  do trabalho não commitado do checkout, essas alterações vão junto, arquivos
  não rastreados incluídos.
- **Rode uma sessão confinada.** Um agente pode abrir dentro de uma sandbox do
  sistema operacional: um home novo e vazio contendo apenas o estado do
  próprio provedor, o resto da máquina somente leitura e acesso de escrita ao
  checkout para o qual ele foi aberto. Suas chaves ssh, suas credenciais de
  nuvem e todos os outros repositórios do disco simplesmente não estão lá: o
  contrapeso de deixar um agente pular os pedidos de permissão. No Linux
  roda o bubblewrap e no macOS o `sandbox-exec`; não é uma barreira contra
  código hostil, e o [`docs/ceilings.md`](docs/ceilings.md) diz o que ela não
  impede.
- **Revise o diff onde você lê.** Um dock com CodeMirror mostra as alterações
  em andamento ao lado de uma árvore de arquivos ao vivo, recolhe as que só
  moveram espaço em branco quando você pede e exibe uma imagem ou um PDF
  alterado antes e depois. Clique com o botão direito em uma seleção para
  comentar sobre aquelas linhas; o lote é colado na sessão como um único
  prompt, sem enviar. A aba Código navega pelo checkout e pesquisa o texto dos
  arquivos.
- **Entregue o pull request daqui.** Liste os pull requests abertos do
  repositório, faça checkout de um em uma worktree própria, depois leia o
  diff, revise inline e faça o merge, com os métodos que a branch base de fato
  aceita. Em uma branch que ainda não tem pull request, **Criar com agente**
  pede à sessão dessa branch que abra um, e nada é enviado até você apertar
  Enter.

Além disso: [temas](docs/themes.md) que você importa como JSON ou instala a
partir de um repositório git, uma paleta `Ctrl`/`Cmd`+`K` que salta por nome ou
pelo que foi dito na conversa, uma notificação do desktop quando uma sessão está
esperando por você e `lich rage` / `lich doctor` para quando ele nem inicia.

O desenvolvimento é ativo: bugs e pedidos de funcionalidade vão em
[Issues](https://github.com/omartelo/lich/issues), e o que mudou em cada
versão está no [CHANGELOG.md](CHANGELOG.md).

## Suporte por provedor

Todo provedor é iniciado em um terminal de verdade, retomado pelo id da
conversa, confinado pela mesma sandbox e recebe o mesmo caminho para alcançar
suas outras sessões. O que muda é o que o lich consegue *ler* de volta de uma
sessão, porque ele lê o que cada CLI registra, e não há duas que registrem as
mesmas coisas.

| O que você tem | Claude Code | Codex | Antigravity | opencode | oh-my-pi | Crush | Cursor CLI | Kiro CLI |
| --- | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: |
| Janela de contexto no rodapé | sim | sim | não | não | não | não | não | sim |
| Custo no rodapé | sim | sim | não | sim | sim | sim | não | créditos |
| Quanto do seu plano ainda resta | sim | sim | sim | só OpenCode Go | não | não | sim | não |
| Um turno interrompido pelo limite de uso do seu plano é retomado quando o limite renova | sim | sim | não | tentativas próprias | tentativas próprias | não | não | não |
| Spinner enquanto um turno roda, anel quando termina | sim | sim | sim | sim | sim | não | sim | sim |
| Um pull request aberto pelo agente aparece no card quando o turno termina | sim | sim | sim | sim | sim | não, só quando a janela volta a ter foco | sim | sim |
| Repinta no novo visual quando você alterna o lich entre um tema claro e um escuro | sim | não | não | sim | sim | não | sim | não |
| Sino quando o agente está bloqueado esperando você | sim | sim | não | sim | não | não | não | não |
| Ctrl+Shift+Y salta para a próxima sessão que espera por você | sim | sim | não | sim | não | não | não | não |
| O card avisa quando a conversa está sendo compactada | sim | não | não | não | não | não | não | não |
| Filtro da barra lateral por estado: Aguardando, Rodando, Não lidas | sim | sim | sem Aguardando | sim | sem Aguardando | não, sempre Ociosas | sem Aguardando | sem Aguardando |
| Executado pelo seu agente dentro de outra sessão (`claude -p`, `cursor-agent -p`), fica fora do card dessa sessão | sim | sim | sim | sim | sim | sim | sim | não |
| Máquina mantida fora da suspensão por inatividade enquanto um turno roda | sim | sim | sim | sim | sim | não | sim | sim |
| **Último turno** da aba Revisão, com o resumo do agente | sim | sim | sim | sim | sim | não | sim | sim |
| Pesquisar a conversa pela paleta | sim | sim | sim | sim | sim | sim | não | sim |
| Retomar pela aba Histórico da paleta uma conversa iniciada fora do lich | sim | sim | sim | sim | sim | Linux; macOS e Windows não testados | sim | sim |
| Um paste longo pode chegar como texto editável em vez de um marcador dobrado (Configurações › Provedores, desligado por padrão) | sim | não | não | sim | não medido | não, vira um anexo `paste_N.txt` | não | sim |
| Fazer fork de uma conversa para uma nova worktree | sim | sim | não | sim | não | não | não | não |
| Abrir uma sessão com um esforço de raciocínio escolhido | sim | sim | sim | não | sim | não | no nome do modelo | sim |
| Abrir uma sessão com o ultracode ligado | sim | não | não | não | não | não | não | não |
| Várias contas lado a lado como perfis, cada uma com seu login completo, e uma sessão movida para outra conta continuando no mesmo card (na janela nativa, experimental: Configurações › Provedores) | sim | sim, com plugins próprios em cada perfil e as configurações do Pessoal compartilhadas por todos | não | só no Linux, com login feito em um card de terminal; uma sessão não pode mudar de perfil, já que as conversas dela ficam no banco de dados que guarda o login | sim, com login feito em um card de terminal e as configurações compartilhadas por todos os perfis | não medido | só no Linux, com login feito em um card de terminal, e `mcp.json`, hooks e skills compartilhados por todos os perfis | só no Linux, com login feito em um card de terminal e as conversas compartilhadas por todos os perfis |
| Conduzir uma sessão em execução por `lich control` ou por um agente: prompt, parar, modelo, esforço, slash command | sim | não | não | não | não | não | não | não |
| Uma mensagem de outra sessão chega sem ser digitada no terminal | sim | não | não | não | não | não | não | não |
| Um subagente que o agente inicia roda como uma sessão do lich com card próprio, no checkout da sessão que o pediu, devolve o relatório completo e fecha ao terminar; os subagentes dele também ganham cards, um nível abaixo | sim | não | não | não | não | não | não | não |
| Fazer uma pergunta lateral a uma sessão em execução sem pará-la (`lich ask`) | sim | não | não | não | não | não | não | não |
| Contexto, custo e uso do plano como a própria CLI mediu | sim | não | não | não | não | não | não | não |
| A status line do terminal mostra a quem a sessão deve uma resposta, de quem ela espera uma e as respostas prontas para coletar | sim | não | não | não | não | não | não | não |
| Delegar em privado a partir de um subagente ou de uma etapa de workflow | sim | sim | sim | sim | sim | sim | sim | sim |
| Uma sessão retomada mantém o modelo e o esforço em que estava | só o modelo | sim | não | modelo | sim | não | modelo | modelo; esforço não testado |
| Um resultado continua esperando por você depois que você interrompe a espera por ele | sim | sim | sim | sim | não | sim | sim | sim |
| Uma tarefa que você entrega a outra sessão volta como "sem resposta" assim que esse agente encerra o turno sem responder | sim | sim | sim | sim | sim | não, espera o prazo | sim | sim |
| As próprias mensagens do lich para o agente (tarefas entregues, resultados prontos, avisos) no idioma de prompt que você escolher | sim | sim | sim | sim | sim | sim | sim | sim |
| Avisado na criação, no idioma de prompt, para distribuir o trabalho como sessões do lich em vez de subagentes ocultos | sim | não | não | não | sim | não | não | não |

O Crush não informa nem o início nem o fim de um turno, e é aí que seis dessas
linhas somem de uma vez: nada abre uma janela para o spinner do card, para o
sino, para o último turno da aba Revisão, para a retenção que mantém a máquina
acordada, para o pull request que ele abriu chegar ao card, nem para avisar
quem lhe entregou uma tarefa que o turno terminou sem resposta. A aba Revisão
diz isso na própria sessão, em vez de deixar você notar que o interruptor
nunca apareceu. O Kiro CLI mede o gasto em créditos e não em dólares, então o
rodapé dele é o único lugar onde esse valor pode ser lido.
O plano do Antigravity é lido perguntando ao próprio `agy` (1.1.11 ou mais
novo; um anterior gastaria um turno para responder, então o lich não pergunta
a ele); o do opencode só quando você entrou em uma assinatura do OpenCode Go,
já que fora isso ele cobra das suas próprias chaves de API; o do Cursor CLI
vem do login que o `cursor-agent login` gravou, nunca do editor Cursor.
oh-my-pi, Crush e Kiro CLI não informam nenhum plano que o lich consiga ler.
Conduzir uma sessão em execução exige o Claude Code 2.1.280 ou mais novo com o
lich-plugin 0.15.0: isso passa por um mod do Claude Code, e nenhuma outra CLI
tem algo rodando por dentro a que o lich possa entregar um comando. O mesmo
mod, com o lich-plugin 0.16.0, informa o que o Claude Code mediu sobre si
mesmo; o rodapé de todas as outras CLIs, e o de uma sessão do Claude Code sem
ele, mostra o que o lich lê do que a CLI registrou.
O grupo **Fora do lich** da aba Histórico lista conversas que rodaram em um
dos checkouts dos seus projetos e que o lich não guarda. Uma sessão que o lich
abriu e você fechou de vez fica de fora dele, exceto uma fechada antes da
0.62.0: o lich distingue as conversas do próprio Claude Code pelo nome com que
as inicia, e as outras sete CLIs não registram nada que permita distingui-las,
então as delas aparecem ali uma vez.

Toda lacuna é deliberada, nenhuma é o lich escondendo algo que a CLI informa, e
o [`docs/ceilings.md`](docs/ceilings.md) diz o que foi medido por trás de cada
uma.

## Instalação

Uma linha: detecta sua distro, verifica o checksum e instala o pacote nativo e
suas dependências pelo seu gerenciador de pacotes:

```bash
curl -fsSL https://raw.githubusercontent.com/omartelo/lich/main/install.sh | sh
```

| Plataforma | Como obter | Necessário em tempo de execução |
| --- | --- | --- |
| **Linux** | o `install.sh` acima, ou o AUR [`lich-bin`](https://aur.archlinux.org/packages/lich-bin) (`yay -S lich-bin`) | `zenity`, além de glibc 2.34 ou mais nova para a janela que vem no pacote (Debian 12, Ubuntu 22.04, RHEL 9 e posteriores) |
| **macOS** | `brew install --cask lichdotdev/tap/lich` | nada, a janela vem no app, tanto em Apple Silicon quanto em Intel |
| **Windows** | instalador ou zip portátil em [Releases](https://github.com/omartelo/lich/releases), ou Scoop: `scoop install https://github.com/omartelo/lich/releases/latest/download/lich.json` | nada, a janela vem em cada um |

Pacotes manuais por distro, o tarball do Linux e o zip portátil do Windows:
[INSTALL.md](INSTALL.md). Os binários de macOS e Windows não são assinados: o
Gatekeeper e o SmartScreen avisam até que a notarização/assinatura cheguem. As
instalações pelo Homebrew evitam o aviso do Gatekeeper; um download feito pela
página de Releases precisa ter a flag de quarentena removida à mão.
No macOS o cask instala o `Lich.app`, então o lich tem ícone próprio em
`/Applications` e no Dock enquanto roda. Quem atualiza da fórmula antiga
precisa rodar `brew uninstall lich` antes; o [INSTALL.md](INSTALL.md) explica
por quê.

## Primeiros passos

1. **Instale** e abra o `lich`.
2. **Abra um projeto**: o `+` na faixa de abas lista o que você fechou há
   pouco e abre o seletor de pastas do seu sistema; aponte para um
   repositório git.
3. **Aponte o lich para o seu agente**: na primeira execução ele lista os
   agentes que encontrou na sua máquina; em Configurações › Provedores você
   define o caminho de cada binário e escolhe o padrão. Um projeto pode
   seguir o padrão ou escolher outro provedor na mesma tela.
4. **Inicie uma sessão**: *Nova sessão* cria um terminal rodando o seu agente
   no projeto. O cabeçalho de cada checkout também tem um menu `+` para abrir
   qualquer provedor ativado ou um terminal simples exatamente naquele
   checkout; clique no próprio cabeçalho para recolher ou expandir as sessões
   dele.
5. **Crie uma branch em uma worktree** *(opcional)*: crie uma a partir de
   qualquer branch base; o lich a popula e te leva a uma sessão nova.

## Configuração

- **Provedores**: defina o caminho do binário de cada provedor e o padrão para
  todos os projetos em Configurações › Provedores. A linha do projeto aberto
  ali pode sobrescrever essa escolha; **Limpar** remove a sobrescrita, de modo
  que mudanças posteriores no padrão passam a valer automaticamente. Sem
  caminho definido, a sessão roda o primeiro binário do provedor no `PATH` do
  seu shell de login; na janela nativa, o bloco Binário mostra esse arquivo e
  a versão dele e lista qualquer outra versão que o `PATH` tenha, sobre a qual
  o `lich doctor` também avisa. Para escolher uma, defina o caminho dela em
  Binário (a configuração `provider.<id>.bin`, por exemplo
  `provider.codex.bin`).
  A seção de um provedor abre com quanto do seu plano ainda resta, quando o
  lich consegue ler. O que o rodapé mostra se define em Configurações ›
  Aparência arrastando os itens dele; o indicador de custo começa oculto,
  já que esse valor só faz sentido quando você é cobrado por token.
- **Worktrees**: o `.lich/setup-worktree.sh` no checkout do projeto roda no
  terminal de uma nova worktree antes do agente; o diálogo Nova worktree o
  mostra e oferece uma sugestão detectada quando o repositório não traz
  nenhum. Um arquivo `.worktreeinclude` ajusta quais arquivos ignorados pelo
  git são copiados.
- **Hooks de sessão**: com o
  [plugin do lich](https://github.com/lichdotdev/lich-plugin) instalado em
  Configurações, uma sessão dá título ao próprio card e atualiza o git no
  instante em que grava um arquivo.
- **Estabilidade**: quais partes do lich são seguras para automatizar por
  script, e quais podem mudar em qualquer versão, está escrito em
  [docs/stability.md](docs/stability.md).

## Privacidade e atualizações

Tudo roda na sua máquina. Sem conta, sem login, sem telemetria: o backend é um
listener de loopback autenticado por token. Por conta própria, o lich só vai à
rede para o seguinte, e nada além disso:

- **Atualizações**: uma verificação de versão no GitHub Releases na
  inicialização e a cada hora, e a lista de releases do plugin do lich no
  GitHub na inicialização.
- **Uso do plano**: o endpoint de uso de cada provedor (Anthropic, OpenAI,
  Cursor, OpenCode Go), com o login que a CLI da sessão já usa, no máximo a
  cada cinco minutos. O plano do Antigravity é lido do próprio `agy`.
- **Preços**: a tabela pública de preços do LiteLLM no GitHub, uma vez para um
  modelo que a tabela embutida não conhece.

A build do instalador do Windows se atualiza no próprio lugar; uma instalação
por Homebrew, Scoop, AUR ou pacote do Linux se atualiza pelo gerenciador de
pacotes, e o lich te entrega o comando.
Configurações › Ajuda diz o que o arquivo de log contém (caminhos, nomes de
projeto e de branch, seu login do gh, nunca um token de sessão) antes de você
anexá-lo a um relatório de bug, e o `lich rage` reúne esse relatório em um
único arquivo sem enviar nada dele.

## Compilar a partir do código-fonte

Backend em Go puro (Go 1.27, `CGO_ENABLED=0`) servindo um frontend embutido em
React 18 / TypeScript / Vite por um listener de loopback autenticado por token
(RPC HTTP + WebSockets). Os terminais são xterm.js com o addon WebGL; as
superfícies de código e diff são CodeMirror 6. O shell Chromium é um registro
de decisão: [`docs/chromium-shell.md`](docs/chromium-shell.md). Os
pré-requisitos são **Go 1.27.0+**, **Node + pnpm** e
**[Task](https://taskfile.dev)**, sem toolchain C para o binário Go. A janela
(`shell/`, Rust sobre CEF) acrescenta um toolchain Rust, além das bibliotecas
de desenvolvimento do Chromium no Linux e do MSVC no Windows;
o [CONTRIBUTING.md](CONTRIBUTING.md) as lista.

```bash
task dev      # hot-reload dev mode (Vite on :9245)
task build    # production binary -> bin/lich
task run      # build + run
task test     # Go + frontend suites
```

Empacote um release do Linux localmente (precisa do
[nfpm](https://nfpm.goreleaser.com/)):

```bash
task package   # .deb + .rpm + Arch .pkg.tar.zst in bin/
```

Adicionar outra CLI de agente às oito que o lich roda é a mudança que cai em
uma dúzia de arquivos de dois repositórios:
o [`docs/adding-a-provider.md`](docs/adding-a-provider.md) é o mapa.

## Patrocinadores

O lich é escrito e mantido por uma pessoa. Patrocinar paga o tempo dedicado a
ele e mantém o projeto independente: não existe plano pago do app e não vai
existir.

[**Seja um patrocinador**](https://github.com/sponsors/omartelo)

<!-- sponsor-names: monthly sponsors go here -->

### Apoiadores

<!-- backers: one-time supporters go here -->

Ninguém ainda.

## Licença

[AGPL-3.0-only](LICENSE) © 2026 omartelo

O lich é software livre: você pode usá-lo, estudá-lo, modificá-lo e
redistribuí-lo nos termos da GNU Affero General Public License v3. Qualquer
derivado distribuído ou servido em rede deve ser publicado sob a mesma
licença. Os releases até a v0.9.0, inclusive, continuam sob a licença MIT.
