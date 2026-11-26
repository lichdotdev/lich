-- The editor's side inside the embedded Neovim: each lich editor holds one
-- buffer here, and every change to its lines, cursor, mode, visual region and
-- view goes back to lich, which draws them.
local chan = ...
local M = {}
_G.lich_editor = M

local api = vim.api
local owned = {}
-- seen is the last click or wheel of lich's carried out, sent with every state.
local seen = 0
-- state, defined below, is what lich hears of the current buffer.
local state

local function regions(mode)
  local kind = mode:sub(1, 1)
  if kind ~= 'v' and kind ~= 'V' and kind ~= '\22' then
    return {}
  end
  local out = {}
  for _, r in ipairs(vim.fn.getregionpos(vim.fn.getpos('v'), vim.fn.getpos('.'), { type = kind })) do
    out[#out + 1] = { r[1][2], r[1][3], r[2][2], r[2][3] }
  end
  return out
end

-- searched is the pattern Vim highlights now: the one being typed after / or
-- ? with 'incsearch', else the last search while 'hlsearch' shows it.
local function searched()
  local t = vim.fn.getcmdtype()
  if (t == '/' or t == '?') and vim.o.incsearch then
    return vim.fn.getcmdline()
  end
  if vim.v.hlsearch == 1 and vim.o.hlsearch then
    return vim.fn.getreg('/')
  end
  return ''
end

-- matches are the visible matches of the highlighted search, as row, byte
-- column and byte length.
local function matches(buf)
  local pat = searched()
  if pat == '' then
    return {}
  end
  -- matchbufline follows 'ignorecase' but not 'smartcase', which a search does.
  if vim.o.ignorecase and vim.o.smartcase and pat:find('%u') then
    pat = '\\C' .. pat
  end
  local ok, found = pcall(vim.fn.matchbufline, buf, pat, vim.fn.line('w0'), vim.fn.line('w$'))
  if not ok then
    return {}
  end
  local out = {}
  for _, m in ipairs(found) do
    if #m.text > 0 then
      out[#out + 1] = { m.lnum, m.byteidx, #m.text }
    end
  end
  return out
end

-- subParts splits a :s argument, "/pat/rep/flags", at its unescaped
-- delimiters: the pattern alone while the replacement is not typed yet.
local function subParts(args)
  local d = args:sub(1, 1)
  if d == '' or d:match('[%w%s\\"|]') then
    return nil
  end
  local parts, cur, i = {}, '', 2
  while i <= #args do
    local c = args:sub(i, i)
    if c == '\\' then
      cur, i = cur .. args:sub(i, i + 1), i + 2
    elseif c == d and #parts < 2 then
      parts[#parts + 1], cur, i = cur, '', i + 1
    else
      cur, i = cur .. c, i + 1
    end
  end
  parts[#parts + 1] = cur
  return parts, d
end

local scratch

-- changedSpan is the bytes of new that old lacks on one line: what lies
-- between their common start and common end.
local function changedSpan(old, new)
  local pre = 0
  while pre < #old and pre < #new and old:byte(pre + 1) == new:byte(pre + 1) do
    pre = pre + 1
  end
  local suf = 0
  while suf < #old - pre and suf < #new - pre and old:byte(#old - suf) == new:byte(#new - suf) do
    suf = suf + 1
  end
  return pre, #new - suf - pre
end

-- substitution is the :s being typed, as 'inccommand' shows it: on the
-- visible lines of its range, the matches of its pattern while the
-- replacement is not typed, else the lines it would make, run by Vim on a copy
-- and laid in place of first..last (0-based, last excluded), with the bytes
-- they changed. The buffer is never touched.
local function substitution(buf)
  if vim.fn.getcmdtype() ~= ':' or not vim.o.incsearch then
    return nil
  end
  local ok, cmd = pcall(api.nvim_parse_cmd, vim.fn.getcmdline(), {})
  if not ok or cmd.cmd ~= 'substitute' then
    return nil
  end
  local parts, d = subParts(cmd.args[1] or '')
  if not parts or parts[1] == '' then
    return nil
  end
  local range = cmd.range or {}
  local first = range[1] or vim.fn.line('.')
  local top, bot = math.max(first, vim.fn.line('w0')), math.min(range[2] or first, vim.fn.line('w$'))
  if top > bot then
    return nil
  end
  if #parts == 1 or vim.o.inccommand == '' then
    local pat = parts[1]
    if vim.o.ignorecase and vim.o.smartcase and pat:find('%u') then
      pat = '\\C' .. pat
    end
    local ok2, found = pcall(vim.fn.matchbufline, buf, pat, top, bot)
    local out = {}
    for _, m in ipairs(ok2 and found or {}) do
      if #m.text > 0 then
        out[#out + 1] = { m.lnum, m.byteidx, #m.text }
      end
    end
    return { matches = out }
  end
  local old = api.nvim_buf_get_lines(buf, top - 1, bot, false)
  scratch = scratch and api.nvim_buf_is_valid(scratch) and scratch or api.nvim_create_buf(false, true)
  api.nvim_buf_set_lines(scratch, 0, -1, false, old)
  -- c would ask at every match; the preview shows them all.
  local flags = (parts[3] or ''):gsub('c', '')
  api.nvim_buf_call(scratch, function()
    vim.cmd('silent! keeppatterns keepjumps %s' .. d .. parts[1] .. d .. parts[2] .. d .. flags)
  end)
  local new = api.nvim_buf_get_lines(scratch, 0, -1, false)
  local out = {}
  for i, line in ipairs(new) do
    local col, len = 0, #line
    if #new == #old then
      col, len = changedSpan(old[i], line)
    elseif line == old[i] then
      len = 0
    end
    if len > 0 then
      out[#out + 1] = { top + i - 1, col, len }
    end
  end
  return { first = top - 1, last = bot, lines = new, matches = out }
end

-- parens is the pair the matchparen plugin lights around the cursor, as
-- row, byte column and byte length; none while the plugin is off.
local function parens()
  local out = {}
  for _, m in ipairs(vim.fn.getmatches()) do
    if m.group == 'MatchParen' then
      for k = 1, 8 do
        local p = m['pos' .. k]
        if p then
          out[#out + 1] = { p[1], p[2] - 1, p[3] or 1 }
        end
      end
    end
  end
  return out
end

-- servers are the language servers lich starts for a file the person's
-- config attached none to, by filetype: the first whose command is on the
-- PATH, rooted at the nearest of its markers.
local servers = {
  {
    name = 'gopls',
    fts = { 'go', 'gomod', 'gowork' },
    cmd = { 'gopls' },
    root = { 'go.work', 'go.mod', '.git' },
    install = { { 'go', 'go install golang.org/x/tools/gopls@latest' } },
  },
  {
    name = 'typescript-language-server',
    fts = { 'typescript', 'typescriptreact', 'javascript', 'javascriptreact' },
    cmd = { 'typescript-language-server', '--stdio' },
    root = { 'tsconfig.json', 'jsconfig.json', 'package.json', '.git' },
  },
  {
    name = 'basedpyright',
    fts = { 'python' },
    cmd = { 'basedpyright-langserver', '--stdio' },
    root = { 'pyproject.toml', 'setup.py', '.git' },
    install = { { 'pipx', 'pipx install basedpyright' }, { 'pip', 'pip install --user basedpyright' } },
  },
  { name = 'pyright', fts = { 'python' }, cmd = { 'pyright-langserver', '--stdio' }, root = { 'pyproject.toml', 'setup.py', '.git' } },
  {
    name = 'rust-analyzer',
    fts = { 'rust' },
    cmd = { 'rust-analyzer' },
    root = { 'Cargo.toml', '.git' },
    install = { { 'rustup', 'rustup component add rust-analyzer' } },
  },
  { name = 'clangd', fts = { 'c', 'cpp', 'objc', 'objcpp' }, cmd = { 'clangd' }, root = { 'compile_commands.json', '.clangd', '.git' } },
  { name = 'lua-language-server', fts = { 'lua' }, cmd = { 'lua-language-server' }, root = { '.luarc.json', '.git' } },
  { name = 'jdtls', fts = { 'java' }, cmd = { 'jdtls' }, root = { 'pom.xml', 'build.gradle', 'build.gradle.kts', '.git' } },
}

-- serverFor is the server lich would start for filetype ft and whether its
-- command is on the PATH; nil for a filetype it knows none for.
local function serverFor(ft)
  local first
  for _, s in ipairs(servers) do
    if vim.tbl_contains(s.fts, ft) then
      if vim.fn.executable(s.cmd[1]) == 1 then
        return s, true
      end
      first = first or s
    end
  end
  return first, false
end

-- started are the clients lich started, which it turns completion on for,
-- and failed the server each buffer's ended with an error before attaching.
local started = {}
local failed = {}

-- How long a file waits for the person's config to attach a server before
-- lich starts its own: lspconfig and vim.lsp.enable attach on FileType.
local attachWait = 1500

-- stderrOf is the first line the server name last wrote to its stderr, as
-- Neovim's lsp.log keeps it: "rpc", the client's name, "stderr", the text.
local function stderrOf(name)
  local f = io.open(vim.lsp.get_log_path(), 'rb')
  if not f then
    return ''
  end
  local size = f:seek('end')
  f:seek('set', math.max(size - 65536, 0))
  local tail = f:read('*a')
  f:close()
  local last = ''
  for msg in tail:gmatch('"rpc"\t"' .. vim.pesc(name) .. '"\t"stderr"\t"(.-)"\n') do
    last = msg
  end
  return vim.trim((last:gsub('\\n.*', '')))
end

local reasons = {}

local function autoLsp(buf, wait)
  vim.defer_fn(function()
    if not api.nvim_buf_is_valid(buf) or #vim.lsp.get_clients({ bufnr = buf }) > 0 then
      return
    end
    local s, found = serverFor(vim.bo[buf].filetype)
    if not s or not found then
      return
    end
    local id = vim.lsp.start({
      name = s.name,
      cmd = s.cmd,
      root_dir = vim.fs.root(buf, s.root) or vim.fs.dirname(api.nvim_buf_get_name(buf)),
      on_exit = function(code)
        if code ~= 0 and #vim.lsp.get_clients({ bufnr = buf }) == 0 then
          failed[buf] = s.cmd[1]
          vim.schedule(function()
            reasons[buf] = stderrOf(s.name)
            state()
          end)
        end
      end,
    }, { bufnr = buf })
    if id then
      started[id] = true
    end
  end, wait or attachWait)
end

-- retry starts buf's server again where none is attached: one installed
-- since it was found missing or failed.
function M.retry(buf)
  if api.nvim_buf_is_valid(buf) and #vim.lsp.get_clients({ bufnr = buf }) == 0 then
    failed[buf], reasons[buf] = nil, nil
    autoLsp(buf, 0)
    state()
  end
end

-- installOf is the command that installs s, through the first of its tools
-- on the PATH; '' for none.
local function installOf(s)
  for _, way in ipairs(s.install or {}) do
    if vim.fn.executable(way[1]) == 1 then
      return way[2]
    end
  end
  return ''
end

-- lsp is what the strip says of the buffer's servers: the names attached,
-- the server lich knows for its filetype and could not find, or the one it
-- started that exited at once with the first line of its error, and the
-- command that installs it.
local function lsp(buf)
  local names = {}
  for _, c in ipairs(vim.lsp.get_clients({ bufnr = buf })) do
    names[#names + 1] = c.name
  end
  local missing, install = '', ''
  if #names == 0 then
    local s, found = serverFor(vim.bo[buf].filetype)
    if s and not found then
      missing = s.cmd[1]
    end
    if s and (missing ~= '' or failed[buf]) then
      install = installOf(s)
    end
  end
  local fail = #names == 0 and failed[buf] or ''
  return { names, missing, fail, install, fail ~= '' and reasons[buf] or '' }
end

function state()
  local buf = api.nvim_get_current_buf()
  if not owned[buf] then
    return
  end
  local mode = api.nvim_get_mode().mode
  local cur = api.nvim_win_get_cursor(0)
  local sub = substitution(buf) or {}
  local preview = sub.lines and { sub.first, sub.last, sub.lines } or {}
  vim.rpcnotify(chan, 'lich_state', buf, mode, cur[1], cur[2], vim.fn.line('w0'), regions(mode),
    sub.matches or matches(buf), preview, parens(), lsp(buf), seen)
end

local group = api.nvim_create_augroup('lich_editor', { clear = true })
api.nvim_create_autocmd({ 'CursorMoved', 'CursorMovedI', 'ModeChanged', 'WinScrolled', 'BufEnter', 'TextChanged', 'TextChangedI', 'CmdlineChanged' }, {
  group = group,
  -- Scheduled: ModeChanged leaves the command line before its command runs,
  -- and :nohlsearch has to have run for the matches to go.
  callback = function() vim.schedule(state) end,
})

-- A yank flashes its text when the person's config lights yanks itself (a
-- TextYankPost of their own, as vim.hl.on_yank), whose highlight lich cannot
-- draw.
api.nvim_create_autocmd('TextYankPost', {
  group = group,
  callback = function()
    local buf = api.nvim_get_current_buf()
    if not owned[buf] or #api.nvim_get_autocmds({ event = 'TextYankPost' }) < 2 then
      return
    end
    local kind = vim.v.event.regtype:sub(1, 1)
    local out = {}
    local ok, found = pcall(vim.fn.getregionpos, vim.fn.getpos("'["), vim.fn.getpos("']"), { type = kind })
    for _, r in ipairs(ok and found or {}) do
      out[#out + 1] = { r[1][2], r[1][3], r[2][2], r[2][3] }
    end
    vim.rpcnotify(chan, 'lich_yank', buf, kind, out)
  end,
})

api.nvim_create_autocmd('LspAttach', {
  group = group,
  callback = function(ev)
    if started[ev.data.client_id] and vim.lsp.completion then
      vim.lsp.completion.enable(true, ev.data.client_id, ev.buf, { autotrigger = true })
    end
    vim.schedule(state)
  end,
})
api.nvim_create_autocmd('LspDetach', { group = group, callback = function() vim.schedule(state) end })

-- The commands that leave a file, and whether each writes it first.
local quits = { quit = false, qall = false, wq = true, wqall = true, xit = true, xall = true, exit = true }

-- A quit typed in lich's buffer closes the file in lich instead of Neovim,
-- which stays up for the other editors: the command line is swapped for
-- lich_editor.quit before it runs.
api.nvim_create_autocmd('CmdlineLeave', {
  group = group,
  callback = function()
    if vim.fn.getcmdtype() ~= ':' or vim.v.event.abort or not owned[api.nvim_get_current_buf()] then
      return
    end
    local ok, cmd = pcall(api.nvim_parse_cmd, vim.fn.getcmdline(), {})
    if ok and quits[cmd.cmd] ~= nil then
      vim.fn.setcmdline(string.format('lua lich_editor.quit(%s, %s)', tostring(quits[cmd.cmd]), tostring(cmd.bang)))
    end
  end,
})

local function fail(msg)
  if not pcall(api.nvim_echo, { { msg, 'ErrorMsg' } }, true, { err = true }) then
    api.nvim_err_writeln(msg)
  end
end

-- quit closes the file in lich: written first when write, refused as Vim
-- refuses it while it holds changes not written, unless bang drops them.
function M.quit(write, bang)
  local buf = api.nvim_get_current_buf()
  if not write and not bang and vim.bo[buf].modified then
    fail('E37: No write since last change (add ! to override)')
    return
  end
  if write then
    vim.cmd('write')
  end
  vim.rpcnotify(chan, 'lich_quit', buf, bang and not write)
end

-- pick chooses item i of the completion menu, as a click on it.
function M.pick(buf, i)
  if api.nvim_get_current_buf() == buf then
    pcall(api.nvim_select_popupmenu_item, i, true, false, {})
  end
end

-- saved hears that lich wrote lines to the file: the buffer is unmodified
-- unless it changed since.
function M.saved(buf, lines)
  if api.nvim_buf_is_valid(buf) and vim.deep_equal(api.nvim_buf_get_lines(buf, 0, -1, false), lines) then
    vim.bo[buf].modified = false
  end
end

-- open makes the buffer for path holding lines: named as the file, so the
-- filetype and the person's settings for it apply, but written by lich.
function M.open(path, lines, wrap)
  local buf = api.nvim_create_buf(true, false)
  vim.bo[buf].swapfile = false
  api.nvim_buf_set_lines(buf, 0, -1, false, lines)
  pcall(api.nvim_buf_set_name, buf, path)
  vim.bo[buf].buftype = 'acwrite'
  vim.bo[buf].modified = false
  -- A smooth scroll walks the top line to where the wheel set it, and the
  -- editor, which follows every top Neovim reports, would walk back with it.
  vim.b[buf].snacks_scroll = false
  vim.b[buf].minianimate_disable = true
  owned[buf] = true
  api.nvim_buf_attach(buf, false, {
    on_lines = function(_, b, tick, first, last, lastnew)
      vim.rpcnotify(chan, 'lich_lines', b, tick, first, last, api.nvim_buf_get_lines(b, first, lastnew, false))
    end,
  })
  api.nvim_create_autocmd('BufWriteCmd', {
    group = group,
    buffer = buf,
    callback = function()
      vim.bo[buf].modified = false
      vim.rpcnotify(chan, 'lich_write', buf)
    end,
  })
  vim.keymap.set('n', 'ZZ', function() M.quit(true, false) end, { buffer = buf })
  vim.keymap.set('n', 'ZQ', function() M.quit(false, true) end, { buffer = buf })
  api.nvim_set_current_buf(buf)
  vim.cmd('filetype detect')
  vim.wo.wrap = wrap
  autoLsp(buf)
  return buf
end

-- focus puts buf in the window, the one the keys that follow go to.
function M.focus(buf)
  if api.nvim_buf_is_valid(buf) and api.nvim_get_current_buf() ~= buf then
    api.nvim_set_current_buf(buf)
  end
  state()
end

-- set replaces buf's lines with the disk's and answers the tick after it:
-- line events up to it are what lich already holds.
function M.set(buf, lines)
  api.nvim_buf_set_lines(buf, 0, -1, false, lines)
  vim.bo[buf].modified = false
  return api.nvim_buf_get_changedtick(buf)
end

function M.close(buf)
  owned[buf] = nil
  if api.nvim_buf_is_valid(buf) then
    api.nvim_buf_delete(buf, { force = true })
  end
end

-- mark is the last click or wheel of lich's, seq, that Neovim has carried out:
-- what it reported before is older than what the editor shows, and lich drops
-- it.
function M.mark(seq)
  seen = seq
  state()
end

-- click is the pointer in lich's editor, its seq'th: the cursor to row, col,
-- or a characterwise selection from the anchor to it.
function M.click(buf, seq, row, col, arow, acol)
  M.focus(buf)
  local mode = api.nvim_get_mode().mode
  local to = string.format('<Cmd>call cursor(%d,%d)<CR>', row, col + 1)
  local done = string.format('<Cmd>lua lich_editor.mark(%d)<CR>', seq)
  if arow == nil then
    if mode:match('^[vV\22]') then
      to = '<Esc>' .. to
    end
    api.nvim_input(to .. done)
    return
  end
  local from = string.format('<Cmd>call cursor(%d,%d)<CR>', arow, acol + 1)
  local leave = mode == 'n' and '' or '<Esc>'
  api.nvim_input(leave .. from .. 'v' .. to .. done)
end

-- scroll is the wheel in lich's editor, its seq'th: line top at the window's
-- top, the cursor kept inside the window as Vim keeps it.
function M.scroll(buf, seq, top)
  if api.nvim_get_current_buf() == buf and vim.fn.line('w0') ~= top then
    -- The cursor comes along inside 'scrolloff', as the wheel brings it in
    -- Vim; left outside the window, Vim would scroll back to it.
    local height = api.nvim_win_get_height(0)
    local so = math.min(vim.wo.scrolloff, math.floor((height - 1) / 2))
    local last = math.min(top + height - 1, api.nvim_buf_line_count(buf))
    local lnum = math.min(math.max(vim.fn.line('.'), top + so), math.max(last - so, top))
    vim.fn.winrestview({ topline = top, lnum = lnum })
  end
  M.mark(seq)
end

-- extra is the grid's lines the window does not get, the status line's and
-- the command line's, which the UI's height has to add to the text's rows.
function M.extra()
  return vim.o.lines - api.nvim_win_get_height(0)
end
