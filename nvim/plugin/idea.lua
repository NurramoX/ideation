if vim.g.loaded_idea then return end
vim.g.loaded_idea = true

local group = vim.api.nvim_create_augroup("idea", {})

vim.api.nvim_create_autocmd("BufReadCmd", {
  group = group,
  pattern = "idea://*",
  desc = "Load an idea from the idea CLI",
  callback = function(ev) require("idea").read(ev.buf) end,
})

vim.api.nvim_create_autocmd("BufWriteCmd", {
  group = group,
  pattern = "idea://*",
  desc = "Write an idea through the idea CLI",
  callback = function(ev) require("idea").write(ev.buf, ev.match) end,
})

vim.api.nvim_create_user_command("NewIdea", function(opts) require("idea").new(opts.args) end, {
  nargs = "+",
  desc = "Capture a new idea with this title and open it",
})
