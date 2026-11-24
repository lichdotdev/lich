-- The editor's side inside the embedded Neovim: each lich editor holds one
-- buffer here, and every change to its lines, cursor, mode, visual region and
-- view goes back to lich, which draws them.
local chan = ...
local M = {}
_G.lich_editor = M

local api = vim.api
local owned = {}

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

local function state()
  local buf = api.nvim_get_current_buf()
  if not owned[buf] then
    return
  end
  local mode = api.nvim_get_mode().mode
  local cur = api.nvim_win_get_cursor(0)
  vim.rpcnotify(chan, 'lich_state', buf, mode, cur[1], cur[2], vim.fn.line('w0'), regions(mode), matches(buf))
end

local group = api.nvim_create_augroup('lich_editor', { clear = true })
api.nvim_create_autocmd({ 'CursorMoved', 'CursorMovedI', 'ModeChanged', 'WinScrolled', 'BufEnter', 'TextChanged', 'TextChangedI', 'CmdlineChanged' }, {
  group = group,
  -- Scheduled: ModeChanged leaves the command line before its command runs,
  -- and :nohlsearch has to have run for the matches to go.
  callback = function() vim.schedule(state) end,
})

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
  return buf
end

-- focus puts buf in the window, the one the keys that follow go to.
function M.focus(buf)
  if api.nvim_buf_is_valid(buf) and api.nvim_get_current_buf() ~= buf then
    api.nvim_set_current_buf(buf)
  end
  state()
end

-- feed types keys, in key notation, into buf.
function M.feed(buf, keys)
  M.focus(buf)
  api.nvim_input(keys)
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

-- click is the pointer in lich's editor: the cursor to row, col, or a
-- characterwise selection from the anchor to it.
function M.click(buf, row, col, arow, acol)
  M.focus(buf)
  local mode = api.nvim_get_mode().mode
  local to = string.format('<Cmd>call cursor(%d,%d)<CR>', row, col + 1)
  if arow == nil then
    if mode:match('^[vV\22]') then
      to = '<Esc>' .. to
    end
    api.nvim_input(to)
    return
  end
  local from = string.format('<Cmd>call cursor(%d,%d)<CR>', arow, acol + 1)
  local leave = mode == 'n' and '' or '<Esc>'
  api.nvim_input(leave .. from .. 'v' .. to)
end

-- scroll is the wheel in lich's editor: line top at the window's top, the
-- cursor kept inside the window as Vim keeps it.
function M.scroll(buf, top)
  if api.nvim_get_current_buf() == buf and vim.fn.line('w0') ~= top then
    vim.fn.winrestview({ topline = top })
    state()
  end
end

-- extra is the grid's lines the window does not get, the status line's and
-- the command line's, which the UI's height has to add to the text's rows.
function M.extra()
  return vim.o.lines - api.nvim_win_get_height(0)
end
