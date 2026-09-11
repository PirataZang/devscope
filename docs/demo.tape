# DevScope · roteiro de gravação do GIF do README
#
# Gera docs/images/demo.gif a partir do binário de verdade.
# Precisa do VHS (mesma turma do Bubble Tea):
#
#   go install github.com/charmbracelet/vhs@latest
#   vhs docs/demo.tape
#
# Depois é só trocar o bloco de texto do topo do README por:
#   <p align="center"><img src="docs/images/demo.gif" alt="DevScope em uso" width="100%"></p>

Output docs/images/demo.gif

Set Shell        bash
Set FontSize     14
Set Width        1400
Set Height       760
Set Padding      16
Set Theme        "Tokyo Night"
Set TypingSpeed  90ms
Set PlaybackSpeed 1.0

Hide
Type "clear" Enter
Show

# --- abre e deixa a lista respirar ---
Type "devscope"  Sleep 400ms  Enter
Sleep 4s

# --- passeia pelos projetos (a onda de status anima) ---
Down  Sleep 900ms
Down  Sleep 900ms
Up    Sleep 700ms

# --- filtro ao vivo ---
Type "/"          Sleep 500ms
Type "laravel"    Sleep 1500ms
Escape            Sleep 700ms

# --- entra no projeto: Git ---
Enter  Sleep 3s
Right  Sleep 1s
Down   Sleep 800ms
Down   Sleep 800ms

# --- containers ---
Tab    Sleep 3s
Down   Sleep 800ms
Down   Sleep 800ms

# --- ajuda, que mostra a paleta dos grupos ---
Type "?"  Sleep 2s
Type "2"  Sleep 2s
Type "5"  Sleep 2s
Escape    Sleep 800ms

# --- volta e sai ---
Escape  Sleep 1s
Escape  Sleep 1s
Type "q"
Sleep 1s
