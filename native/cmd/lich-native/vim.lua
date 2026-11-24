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

local function state()
  local buf = api.nvim_get_current_buf()
  if not owned[buf] then
    return
  end
  local mode = api.nvim_get_mode().mode
  local cur = api.nvim_win_get_cursor(0)
  vim.rpcnotify(chan, 'lich_state', buf, mode, cur[1], cur[2], vim.fn.line('w0'), regions(mode))
end

local group = api.nvim_create_augroup('lich_editor', { clear = true })
api.nvim_create_autocmd({ 'CursorMoved', 'CursorMovedI', 'ModeChanged', 'WinScrolled', 'BufEnter', 'TextChanged', 'TextChangedI' }, {
  group = group,
  callback = function() state() end,
})

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
