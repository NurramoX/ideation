-- Edit ideas as `idea://<id>` buffers through the `idea` CLI (spec §10).
local M = {}

local config = { cmd = "idea" }

--- @param opts? { cmd?: string } cmd: the idea binary (default "idea")
function M.setup(opts)
  config = vim.tbl_extend("force", config, opts or {})
end

local function err(msg) vim.notify(msg, vim.log.levels.ERROR) end

--- Runs the CLI synchronously. Returns the completed result, or nil after
--- reporting a failure (the CLI's stderr, or why it could not start).
local function run(args, stdin)
  local ok, res = pcall(function()
    return vim.system(vim.list_extend({ config.cmd }, args), { stdin = stdin, text = true }):wait()
  end)
  if not ok then
    err("idea: " .. tostring(res))
    return nil
  end
  if res.code ~= 0 then
    local msg = vim.trim(res.stderr or "")
    err(msg ~= "" and msg or ("idea " .. args[1] .. " exited " .. res.code))
    return nil
  end
  return res
end

local function id_of(name) return name:match("^idea://(%d+)$") end

--- BufReadCmd: loads the idea's body byte-exact into buf.
function M.read(buf)
  local name = vim.api.nvim_buf_get_name(buf)
  local id = id_of(name)
  if not id then return err(("%s: the part after idea:// must be a decimal id"):format(name)) end
  local res = run({ "show", "--json", id })
  if not res then return end
  local idea = vim.json.decode(vim.split(res.stdout, "\n", { plain = true })[1])

  local eol = idea.body:sub(-1) == "\n"
  local lines = vim.split(eol and idea.body:sub(1, -2) or idea.body, "\n", { plain = true })

  local bo = vim.bo[buf]
  local undolevels = bo.undolevels
  bo.undolevels = -1 -- loading is not an undoable change
  vim.api.nvim_buf_set_lines(buf, 0, -1, true, lines)
  bo.undolevels = undolevels
  bo.fileformat = "unix"
  bo.eol = eol
  bo.fixeol = eol
  bo.buftype = "acwrite"
  bo.swapfile = false
  bo.modified = false
  vim.b[buf].idea_version = idea.version
  vim.b[buf].idea_title = idea.title
  bo.filetype = "markdown"
end

--- BufWriteCmd: writes buf to the idea named by target, guarded by the
--- Version it was read at. Never forces; on failure buf stays modified.
function M.write(buf, target)
  local id = id_of(target)
  if not id then return err(("%s: the part after idea:// must be a decimal id"):format(target)) end
  local version = vim.b[buf].idea_version
  if target ~= vim.api.nvim_buf_get_name(buf) or not version then
    return err(("%s: only the buffer loaded from it can be written there"):format(target))
  end
  local body = table.concat(vim.api.nvim_buf_get_lines(buf, 0, -1, true), "\n")
  if vim.bo[buf].eol then body = body .. "\n" end
  local res = run({ "write", id, "--version", tostring(version), "--json" }, body)
  if not res then return end
  vim.b[buf].idea_version = vim.json.decode(res.stdout).version
  vim.bo[buf].modified = false
end

--- :NewIdea: captures an idea with this title and opens it.
function M.new(title)
  title = vim.trim(title)
  if title == "" then return err("NewIdea: a title is required") end
  local res = run({ "add", "--", title })
  if not res then return end
  local id = vim.trim(res.stdout)
  if not id:match("^%d+$") then return err("NewIdea: idea add printed no id: " .. id) end
  vim.cmd.edit(vim.fn.fnameescape("idea://" .. id))
end

return M
