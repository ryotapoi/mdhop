# claude -p --output-format stream-json --verbose の出力を
# 「独白 + ツール名・主要引数」だけに整形する jq フィルタ。
#
# 設計:
#   - assistant.text: 独白。方向性が読めるので止める判断の起点になる
#   - assistant.tool_use: ツール名 + 主要引数 1 行
#   - user の tool_result で is_error=true: 失敗時だけ警告表示
#   - その他（thinking 内部、tool_result の成功時など）は捨てる

if .type == "assistant" then
  .message.content[]?
  | if .type == "text" then
      .text
    elif .type == "tool_use" then
      "  -> " + .name +
      (
        if .name == "Bash" then
          ": " + ((.input.command // "") | gsub("\n"; " ⏎ ") | .[0:160])
        elif .name == "Edit" or .name == "Read" or .name == "Write" or .name == "NotebookEdit" then
          ": " + (.input.file_path // "")
        elif .name == "Grep" then
          ": " + (.input.pattern // "")
        elif .name == "Glob" then
          ": " + (.input.pattern // "")
        elif .name == "Agent" or .name == "Task" then
          ": " + (.input.description // (.input.subagent_type // ""))
        elif .name == "WebFetch" or .name == "WebSearch" then
          ": " + (.input.url // .input.query // "")
        elif .name == "Skill" then
          ": " + (.skill // .input.skill // "")
        else
          ""
        end
      )
    else
      empty
    end

elif .type == "user" then
  .message.content[]?
  | select(.type == "tool_result" and (.is_error == true))
  | "  !! tool_error: " +
    (
      if (.content | type) == "string" then
        (.content | gsub("\n"; " ⏎ ") | .[0:200])
      elif (.content | type) == "array" then
        ([.content[] | select(.type == "text") | .text] | join(" ") | gsub("\n"; " ⏎ ") | .[0:200])
      else
        ""
      end
    )

else
  empty
end
