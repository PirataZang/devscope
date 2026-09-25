# ASCII &amp; Unicode Art para Terminal e TUI

> Guia prático de caracteres, composição visual e técnicas de animação para aplicações de terminal.
>
> Foco principal: criar animações fluidas, bonitas e eficientes em TUI, especialmente em projetos Go.

---

## 1. ASCII vs Unicode

### ASCII

ASCII utiliza um conjunto pequeno de caracteres e funciona praticamente em qualquer terminal.

Caracteres úteis:

```
!"#$%&'()*+,-./
0123456789
:;<=>?@
ABCDEFGHIJKLMNOPQRSTUVWXYZ
[\]^_`
abcdefghijklmnopqrstuvwxyz
{|}~
```

Para desenhos:

```
@ # $ % & * + = - . , : ; '
/ \ | _ ^ ~
< > ( ) [ ] { }
```

### Unicode

Unicode possui milhares de caracteres e é muito mais interessante para TUI moderna.

Exemplos:

```
┌ ┐ └ ┘ ├ ┤ ┬ ┴ ┼
─ │ ═ ║ ╔ ╗ ╚ ╝
╠ ╣ ╦ ╩ ╬
█ ▓ ▒ ░
▀ ▄ ▌ ▐
● ○ ◉ ◎
★ ☆ ◆ ◇
• · ◦
```

Para terminais modernos, Unicode costuma ser a melhor escolha para interfaces e animações.

---

# 2. Caracteres essenciais

## 2.1 Linhas

```
-  _  =
─  │
━  ┃
═  ║
```

Uso:

```
--------------------
────────────────────
════════════════════
```

Linhas duplas e grossas são úteis para separar regiões da interface.

---

## 2.2 Cantos

```
┌ ┐
└ ┘
╔ ╗
╚ ╝
```

Exemplo:

```
┌──────────────────┐
│                  │
│      DEVScope    │
│                  │
└──────────────────┘
```

---

## 2.3 Junções

```
├ ┤ ┬ ┴ ┼
╠ ╣ ╦ ╩ ╬
```

Exemplo:

```
┌──────────────┬──────────────┐
│ PROJECTS     │ SYSTEM       │
├──────────────┼──────────────┤
│ devscope     │ CPU    23%   │
│ api          │ RAM    61%   │
└──────────────┴──────────────┘
```

---

# 3. Blocos

Os blocos são extremamente importantes para gráficos, barras, sombras e animações.

```
█
▓
▒
░
```

Ordem aproximada de densidade:

```
█ █ █ █
▓▓▓▓▓▓
▒▒▒▒▒▒
░░░░░░
```

Podemos representar um gradiente:

```
█▓▒░
```

E uma barra:

```
████████████████
```

Barra parcialmente preenchida:

```
████████░░░░░░░░
```

---

# 4. Meio-blocos

Caracteres:

```
▀
▄
▌
▐
```

Eles permitem aumentar a resolução visual sem aumentar o número de linhas.

Exemplo:

```
▀▀▀▀▀▀▀▀
████████
```

Também podem ser usados para criar sprites pequenos e animações.

---

# 5. Pontos e partículas

Caracteres simples:

```
.
,
'
`
·
•
∙
◦
○
●
```

São excelentes para:

- estrelas
- poeira
- fogo
- chuva
- neve
- partículas
- faíscas
- partículas de ambiente

Exemplo:

```
     ·        *
          ·
  *            ·
       ·
```

---

# 6. Símbolos de destaque

```
★ ☆
◆ ◇
● ○
■ □
▲ △
▼ ▽
```

Podem representar estados:

```
● RUNNING
○ STOPPED
◆ IMPORTANT
★ ACTIVE
```

---

# 7. Caracteres ASCII para desenho

Os caracteres abaixo são particularmente úteis:

```
@ # $ % & *
+ = - _ .
: ; , '
/ \ | 
( ) [ ] { }
< >
^ ~
```

### Densidade

Uma técnica comum é usar caracteres de diferentes densidades:

```
@#%*+=-:. 
```

Do mais pesado para o mais leve.

Isso pode ser usado para transformar imagens ou criar sombras:

```
@@@@@@
@@%%@@
@%**%@
@*++*@
@+==+@
```

---

# 8. Composição de uma cena

Uma animação de terminal não precisa ser apenas um sprite.

Ela pode ser composta por:

```
BACKGROUND
    ↓
ENVIRONMENT
    ↓
OBJECTS
    ↓
PARTICLES
    ↓
LIGHT / EFFECTS
    ↓
UI
```

Exemplo:

```
       *        ·

             .       *
    /\                ·
   /  \       /\ 
  /____\     /  \
     ||       ||        ·

        🔥
       ███
      █████
     ░████░

────────────────────────────
```

Em TUI real, cada camada pode ser atualizada separadamente.

---

# 9. Animação baseada em Frames

A forma mais simples de animar ASCII é criar vários frames.

Frame 1:

```
  /\
 /  \
```

Frame 2:

```
  /\
 /  \
   \
```

Frame 3:

```
  /\
 /  \
  ||
```

O programa alterna:

```
Frame 1
   ↓
Frame 2
   ↓
Frame 3
   ↓
Frame 2
   ↓
Frame 1
```

Isso cria um ciclo.

---

# 10. FPS

FPS significa Frames Per Second.

Exemplos:

```
5 FPS   → 200 ms por frame
10 FPS  → 100 ms por frame
20 FPS  → 50 ms por frame
30 FPS  → 33 ms por frame
60 FPS  → 16 ms por frame
```

Para animações decorativas de terminal:

```
8–15 FPS
```

normalmente já é suficiente.

Para animações mais fluidas:

```
20–30 FPS
```

é uma faixa interessante.

Não é necessário atualizar tudo a 60 FPS.

---

# 11. Tick

Em vez de pensar somente em FPS, uma TUI pode usar um `tick`.

Exemplo conceitual:

```
tick
 ↓
atualiza estado
 ↓
calcula partículas
 ↓
gera frame
 ↓
renderiza
 ↓
tick novamente
```

Isso separa:

```
STATE
```

de:

```
RENDER
```

e torna o sistema mais previsível.

---

# 12. Frames não precisam redesenhar tudo

Uma animação eficiente pode alterar somente o necessário.

Em vez de:

```
renderizar tudo
renderizar tudo
renderizar tudo
```

podemos manter:

```
estado anterior
estado atual
```

e determinar o que mudou.

Porém, em muitas TUIs modernas, redesenhar a região animada inteira ainda é suficientemente barato.

A escolha depende do tamanho e complexidade da interface.

---

# 13. Movimento horizontal

Considere:

```
★
```

Podemos deslocá-lo:

```
★
```

```
  ★
```

```
    ★
```

```
      ★
```

A posição pode ser:

```
x = x + velocity
```

Quando chegar ao limite:

```
if x >= width {
    x = 0
}
```

Resultado:

```
★──────────────→
```

---

# 14. Movimento vertical

```
  *
```

↓

```
  *
```

↓

```
  *
```

↓

```
  *
```

Matematicamente:

```
y = y + velocity
```

---

# 15. Movimento suave

Movimento simples:

```
x += 1
```

é discreto.

Para criar uma sensação mais natural, podemos utilizar:

```
position += velocity
velocity += acceleration
```

Exemplo:

```
position += velocity
velocity += gravity
```

Isso permite criar:

- queda
- salto
- gravidade
- lançamento
- partículas
- fogo
- fumaça

---

# 16. Bounce

Para fazer um objeto bater na borda:

```
position += velocity

if position <= min {
    velocity *= -1
}

if position >= max {
    velocity *= -1
}
```

Visual:

```
●──────→
```

Depois:

```
──────●
```

Depois:

```
←──────●
```

Isso é útil para:

- bolas
- partículas
- ícones
- pequenos objetos
- efeitos de ambiente

---

# 17. Easing

Movimento linear:

```
0 1 2 3 4 5 6 7 8
```

Easing pode fazer:

```
0 0 1 2 4 6 7 8 8
```

ou:

```
0 2 4 6 7 8 8 8 8
```

Isso cria:

- aceleração
- desaceleração
- entrada suave
- saída suave

Funções comuns:

```
ease-in
ease-out
ease-in-out
```

---

# 18. Animação de partículas

Uma partícula pode ter:

```
x
y
velocityX
velocityY
life
maxLife
symbol
```

Exemplo:

```
type Particle struct {
    X        float64
    Y        float64
    VX       float64
    VY       float64
    Life     float64
    MaxLife  float64
    Symbol   rune
}
```

A cada tick:

```
X += VX
Y += VY
Life -= deltaTime
```

Quando:

```
Life <= 0
```

a partícula pode ser removida.

---

# 19. Partículas com símbolos diferentes

Podemos selecionar:

```
. · • *
```

ou:

```
░ ▒ ▓ █
```

Dependendo da vida da partícula:

```
█
▓
▒
░
.
```

Isso cria uma sensação de desaparecimento.

---

# 20. Sistema de estrelas

Uma estrela pode ter:

```
x
y
brightness
twinkleSpeed
```

Em vez de mover a estrela, alteramos o símbolo:

```
.
·
*
✦
*
·
.
```

Isso cria um efeito de brilho.

Exemplo:

```
      ·           *
            ✦

  *                  ·

         ·
```

---

# 21. Twinkle

Uma técnica simples é utilizar uma função periódica:

```
brightness = sin(time * speed)
```

Depois transformamos o resultado em símbolos:

```
0.0 → .
0.3 → ·
0.6 → *
0.9 → ✦
```

A estrela parece pulsar.

---

# 22. Cometa

Um cometa pode ser representado por:

```
      ·
    ··
  ···
 ★
```

Movendo o objeto:

```
      ·
    ··
  ···
 ★
```

↓

```
         ·
       ··
     ···
   ★
```

A cauda pode ser armazenada como posições anteriores.

```
trail[0] = posição atual
trail[1] = posição anterior
trail[2] = posição anterior
...
```

---

# 23. Fogo

O fogo funciona muito bem com caracteres de densidade.

Exemplo:

```
    .
   ░
  ▒▓▒
 ░▓█▓░
  ▒█▒
   ░
```

Uma versão animada pode alternar os frames:

### Frame 1

```
   ░
  ▒▓▒
 ░▓█▓░
  ▒█▒
```

### Frame 2

```
    ░
   ▒▓
  ░█▓▒
   ▓█
```

### Frame 3

```
   ▒
  ░▓▒
   █▓
  ▒█
```

A mudança de forma cria a animação.

---

# 24. Fumaça

Use partículas lentas:

```
.
·
○
◦
```

Movimento:

```
  .
   ·
    ○
     ◦
```

Com uma pequena variação horizontal:

```
x += sin(time * frequency) * amplitude
y -= velocity
```

Isso cria fumaça oscilante.

---

# 25. Chuva

Partículas:

```
|
/
```

Exemplo:

```
|     /       |
    |      /
 /       |
```

Cada gota pode ter:

```
x
y
velocity
```

Quando chega ao chão:

```
y = 0
```

e retorna ao topo.

---

# 26. Neve

Use:

```
·
*
•
❄
```

A neve pode ter movimento horizontal aleatório:

```
x += sin(time * speed + phase)
y += velocity
```

Isso cria uma queda menos mecânica.

---

# 27. Água

Uma técnica simples:

```
~~~~~~~
≈≈≈≈≈≈≈
~~~~~~~
```

Para criar ondas:

```
~ ~ ~ ~ ~ ~
 ~ ~ ~ ~ ~ ~
```

Podemos alternar:

```
~~~~~~
≈≈≈≈≈≈
```

com:

```
≈≈≈≈≈≈
~~~~~~
```

---

# 28. Árvore animada

Estrutura:

```
       /\
      /██\
     /████\
    /██████\
       ||
       ||
```

Podemos fazer vento alterando os galhos:

```
       /\
      /██\
     /████\
    /██████\
       ||
```

Depois:

```
        /\
       /██\
      /████\
     /██████\
        ||
```

Pequenas alterações por frame já criam sensação de vida.

---

# 29. Folhas

Uma folha pode ter:

```
*
·
❧
```

ou simplesmente:

```
.
*
```

Movimento:

```
leaf.x += leaf.vx
leaf.y += leaf.vy
leaf.vy += gravity
```

Para vento:

```
leaf.vx += sin(time + phase) * wind
```

---

# 30. Animação de personagem

Uma técnica clássica é usar pequenos sprites.

### Idle

```
  O
 /|\
 / \
```

### Passo 1

```
  O
 /|\
 / \
```

### Passo 2

```
  O
 /|\
  /\
```

### Passo 3

```
  O
 /|\
 /\ 
```

Mesmo pequenas mudanças fazem o personagem parecer vivo.

---

# 31. Animação por squash &amp; stretch

Para objetos que pulam:

Normal:

```
  ●
```

Caindo:

```
  ●
```

Impacto:

```
 ━●━
```

Subindo:

```
  ●
```

Isso aumenta bastante a percepção de movimento.

---

# 32. Blink

Para indicadores:

```
●
```

alternando com:

```
○
```

Exemplo:

```
● SERVER ONLINE
○ SERVER ONLINE
● SERVER ONLINE
```

Pode ser utilizado para indicar atividade.

---

# 33. Pulse

Um indicador pode variar:

```
·
•
●
•
·
```

Isso cria um pulso.

Muito útil para:

```
● LIVE
● CONNECTED
● WATCHING
```

---

# 34. Barras animadas

Barra estática:

```
████████░░░░
```

Animada:

```
████░░░░░░░░
```

↓

```
██████░░░░░░
```

↓

```
████████░░░░
```

↓

```
██████████░░
```

Pode representar progresso real ou uma animação decorativa.

---

# 35. Spinner

Spinners são extremamente úteis em TUI.

### Simples

```
-
\
|
/
```

### Unicode

```
⠋
⠙
⠹
⠸
⠼
⠴
⠦
⠧
⠇
⠏
```

### Outro estilo

```
◐
◓
◑
◒
```

---

# 36. Spinner customizado

Exemplo:

```
[    ]
[=   ]
[==  ]
[=== ]
[ == ]
[  = ]
[   ]
```

Pode ser usado em:

```
Building project...
Scanning...
Connecting...
Loading...
```

---

# 37. Marquee

Texto passando horizontalmente:

```
HELLO
```

↓

```
 HELLO
```

↓

```
  HELLO
```

↓

```
   HELLO
```

↓

```
    HELLO
```

Útil para:

- logs
- notificações
- títulos
- status
- mensagens longas

---

# 38. Typewriter

Texto aparece gradualmente:

```
D
DE
DEV
DEVS
DEVSC
DEVSCO
DEVSCOP
DEVSCOPE
```

Também pode ser combinado com cursor:

```
DEVScope_
```

↓

```
DEVScope
```

↓

```
DEVScope_
```

---

# 39. Fade usando caracteres

Em terminal sem alpha real, podemos simular fade:

```
█
▓
▒
░
.
```

Exemplo:

```
████
▓▓▓▓
▒▒▒▒
░░░░
....
```

Isso é extremamente útil para partículas e objetos desaparecendo.

---

# 40. Transição entre dois frames

Frame A:

```
████████
```

Frame B:

```
........
```

Transição:

```
████████
▓▓▓▓▓▓▓▓
▒▒▒▒▒▒▒▒
░░░░░░░░
........
```

Essa técnica pode ser utilizada para:

- entrada
- saída
- desaparecimento
- troca de cena

---

# 41. Double Buffer

Para evitar flickering:

```
STATE
  ↓
FRAME BUFFER
  ↓
RENDER
```

Em vez de imprimir cada caractere individualmente, monte o frame inteiro e envie a saída de uma vez quando possível.

Uma abordagem conceitual:

```
buffer := ""

buffer += "┌────────────┐\n"
buffer += "│ DEVScope   │\n"
buffer += "└────────────┘\n"

render(buffer)
```

---

# 42. Não use sleeps espalhados

Evite arquitetura como:

```
sleep
render
sleep
render
sleep
```

espalhada por toda a aplicação.

Prefira um loop central:

```
for {
    delta := clock.Tick()

    Update(delta)
    View()
}
```

Isso facilita sincronizar:

- partículas
- relógio
- animações
- input
- UI
- efeitos

---

# 43. Delta Time

Para animações independentes da velocidade do computador:

```
position += velocity * deltaTime
```

Assim:

```
30 FPS
```

e:

```
60 FPS
```

podem produzir aproximadamente o mesmo movimento por segundo.

Sem delta time, uma animação pode ficar mais rápida em máquinas mais rápidas.

---

# 44. Animações determinísticas

Quando necessário, utilize um estado próprio:

```
animationFrame
animationTimer
animationSpeed
```

Exemplo:

```
animationTimer += delta

if animationTimer >= frameDuration {
    animationTimer -= frameDuration
    animationFrame++
}
```

Isso é ideal para sprites.

---

# 45. Animações contínuas vs discretas

## Discreta

Troca de frames:

```
frame 0
frame 1
frame 2
frame 3
```

Ideal para:

- personagens
- fogo
- gatos
- animais
- sprites

## Contínua

Usa posições e velocidades:

```
x += velocity * delta
```

Ideal para:

- partículas
- estrelas
- cometas
- chuva
- objetos voando

Uma TUI sofisticada pode combinar as duas.

---

# 46. Sistema de camadas

Uma arquitetura interessante:

```
Scene
│
├── Background
│
├── Stars
│
├── Environment
│
├── Particles
│
├── Characters
│
├── Effects
│
└── UI
```

Cada camada possui:

```
Update()
Render()
```

Exemplo:

```
Background.Update()
Stars.Update()
Particles.Update()
Character.Update()

Background.Render()
Stars.Render()
Particles.Render()
Character.Render()
UI.Render()
```

---

# 47. Área de animação

Em uma TUI, não é necessário animar o terminal inteiro.

Reserve uma área:

```
┌──────────────────────────────────┐
│ DEVScope                         │
├──────────────────────────────────┤
│                                  │
│       ANIMATION AREA             │
│                                  │
│                 *                │
│       /\                         │
│      /  \             ·          │
│                                  │
├──────────────────────────────────┤
│ CPU 23%       RAM 61%            │
└──────────────────────────────────┘
```

Isso é especialmente útil para uma área de ambiente/idle.

---

# 48. Responsividade ao tamanho do terminal

Nunca assuma:

```
width = 80
height = 24
```

Use:

```
width
height
```

do terminal atual.

A animação deve se adaptar:

```
x ∈ [0, width)
y ∈ [0, height)
```

Se o terminal ficar menor:

```
recalculate()
```

Se ficar maior:

```
expand scene
```

---

# 49. Animações proporcionais ao terminal

Para posicionar um objeto no centro:

```
x = width / 2
y = height / 2
```

Para uma posição proporcional:

```
x = int(width * 0.75)
y = int(height * 0.50)
```

Assim a cena permanece equilibrada em diferentes resoluções.

---

# 50. Terminal width e Unicode

Cuidado: nem todo caractere Unicode possui largura visual 1.

Exemplos:

```
A
█
界
```

Podem ter comportamentos diferentes dependendo do caractere, fonte e terminal.

Para TUI, utilize bibliotecas que saibam calcular largura de strings Unicode.

Em Go, isso é especialmente importante quando se usa:

```
lipgloss
```

e componentes de TUI.

---

# 51. Braille

Unicode Braille pode ser utilizado para criar gráficos com resolução muito maior.

Caracteres:

```
⠁ ⠂ ⠄ ⡀
⠈ ⠐ ⠠ ⢀
```

Cada caractere Braille representa uma pequena matriz de pontos.

Isso permite criar:

- gráficos
- partículas
- ondas
- visualizadores
- animações
- desenhos compactos

---

# 52. Por que Braille é interessante?

Uma célula Braille pode representar aproximadamente:

```
2 × 4 pontos
```

em um único caractere.

Portanto:

```
████
```

possui resolução relativamente baixa.

Já Braille permite algo mais próximo de:

```
⣿⣿⣷⣶⣤
```

para representar detalhes menores.

É excelente para animações compactas.

---

# 53. Gráficos usando Braille

Imagine uma onda:

```
      •
   •     •
 •         •
```

Ela pode ser convertida em pontos Braille.

O resultado ocupa poucas células do terminal, mas possui bastante detalhe.

Aplicações:

```
CPU graph
RAM graph
network graph
FPS graph
audio visualizer
activity monitor
```

---

# 54. Braille + partículas

Um sistema pode armazenar partículas em coordenadas de alta resolução:

```
particle.X
particle.Y
```

Depois agrupar os pontos em células Braille.

Fluxo:

```
Particles
    ↓
High-resolution grid
    ↓
Braille encoder
    ↓
Terminal
```

Isso pode gerar efeitos muito mais suaves que caracteres tradicionais.

---

# 55. Caracteres para efeitos de luz

```
.
·
•
*
✦
✧
★
```

Uma estrela pode fazer:

```
.
·
*
✦
*
·
.
```

Um efeito de explosão:

```
      ·
   *  |  *
     \|/
  * --●-- *
     /|\
   *  |  *
      ·
```

---

# 56. Explosão de partículas

Estrutura:

```
Particle {
    position
    velocity
    life
}
```

No impacto:

```
for i := 0; i < count; i++ {
    createParticle()
}
```

Velocidades aleatórias:

```
angle
speed
```

Convertendo:

```
vx = cos(angle) * speed
vy = sin(angle) * speed
```

Depois:

```
x += vx * delta
y += vy * delta
```

---

# 57. Ruído controlado

Aleatoriedade pura pode parecer artificial.

Uma opção melhor é usar:

```
sin()
cos()
noise()
```

Exemplo:

```
x = baseX + sin(time * speed + phase) * amplitude
```

Isso é ótimo para:

- folhas
- água
- fogo
- fumaça
- objetos flutuando
- animais dormindo

---

# 58. Fireflies

Vagalumes podem ser partículas com:

```
position
phase
speed
brightness
life
```

Visual:

```
        ·

   ✦

             ·

      ✧

  ·
```

O brilho pode oscilar:

```
.
·
*
✦
*
·
.
```

E a posição pode mudar lentamente.

---

# 59. Animação de gato dormindo

Um sprite simples:

```
 /\_/\
( -.- )
 > ^ <
```

Estado dormindo:

```
 /\_/\
( -.- )
 > ^ <
    z
   zz
```

Depois:

```
 /\_/\
( o.o )
 > ^ <
```

E finalmente:

```
 /\_/\
( ^.^ )
 > ^ <
```

A animação pode seguir:

```
SLEEP
 ↓
WAKE
 ↓
IDLE
 ↓
SLEEP
```

---

# 60. Animação de respiração

Uma técnica excelente para objetos vivos.

Frame 1:

```
 /\_/\
( -.- )
```

Frame 2:

```
  /\_/\
 ( -.- )
```

Frame 3:

```
 /\_/\
( -.- )
```

O sprite muda ligeiramente de tamanho/posição.

Isso é chamado de **idle animation**.

---

# 61. Animação de olhos

Um personagem pode piscar:

```
( o.o )
```

↓

```
( -.- )
```

↓

```
( o.o )
```

Não é necessário alterar o corpo inteiro.

---

# 62. Delorean / carro em ASCII

Um veículo pode ser construído com:

```
       ______
  ____/|_||_\`.__
 / _          _  \
'-(o)--------(o)-'
```

Os faróis podem alternar:

```
●       ●
```

e:

```
✦       ✦
```

Para um efeito de velocidade:

```
──────
  ────
    ──
```

As linhas se deslocam para trás enquanto o veículo permanece em uma posição.

---

# 63. Ilusão de movimento

Nem sempre o objeto precisa se mover.

Exemplo:

```
CARRO → permanece parado

FUNDO → move
PARTÍCULAS → movem
LUZES → piscam
```

O cérebro interpreta isso como movimento.

Essa técnica reduz a quantidade de cálculo necessária.

---

# 64. Parallax

Crie camadas com velocidades diferentes:

```
CAMADA 1 → 0.1x
CAMADA 2 → 0.3x
CAMADA 3 → 0.7x
CAMADA 4 → 1.0x
```

Exemplo:

```
estrelas distantes  → lentas
montanhas            → médias
partículas           → rápidas
```

Isso cria profundidade.

---

# 65. Ambiente de idle

Para uma tela de descanso, uma composição interessante seria:

```
┌─────────────────────────────────────────┐
│                                         │
│        ·       *             ·          │
│                  ✦                      │
│    *                    ·               │
│                                         │
│             /\                          │
│            /██\        ✦                │
│           /████\                        │
│              ||                         │
│                                         │
│                    /\_/ \               │
│                   ( -.- )               │
│                                         │
│                 ░▒▓█▓▒░                 │
│                                         │
└─────────────────────────────────────────┘
```

Pode conter:

```
céu
estrelas
cometas
árvores
fogo
animais
vagalumes
fumaça
água
```

---

# 66. Regras para uma boa animação TUI

## 1. Evite excesso

Uma animação boa não precisa mexer tudo ao mesmo tempo.

## 2. Use movimento lento para elementos ambientais

Exemplos:

```
estrelas → muito lento
árvores → lento
fumaça → lento
vagalumes → lento
fogo → médio
partículas → rápido
```

## 3. Crie pequenas imperfeições

Movimento completamente linear:

```
x += 1
```

pode parecer robótico.

Combine:

```
velocity
sin
randomness
phase
```

## 4. Preserve o foco da interface

A animação deve ficar em segundo plano quando existe informação importante.

---

# 67. Arquitetura recomendada em Go

Uma arquitetura simples:

```
Animation
│
├── State
│   ├── Time
│   ├── Width
│   ├── Height
│   └── FPS
│
├── Scene
│   ├── Background
│   ├── Particles
│   ├── Objects
│   └── Effects
│
├── Update()
│
└── Render()
```

Exemplo:

```
type Animation struct {
    Width  int
    Height int
    Time   float64
}

func (a *Animation) Update(dt float64) {
    a.Time += dt
}

func (a *Animation) Render() string {
    // monta o frame
    return ""
}
```

---

# 68. Entidades animadas

Uma interface comum:

```
type Entity interface {
    Update(dt float64)
    Render() string
}
```

Então:

```
Star
Fire
Smoke
Cat
Tree
Comet
Firefly
```

podem implementar a mesma interface.

---

# 69. Gerenciador de animações

Exemplo conceitual:

```
type Scene struct {
    Entities []Entity
}

func (s *Scene) Update(dt float64) {
    for _, entity := range s.Entities {
        entity.Update(dt)
    }
}
```

Isso permite adicionar novos elementos sem alterar o loop principal.

---

# 70. Performance

Para uma TUI:

### Evite

```
alocar milhares de strings por frame
```

quando não for necessário.

### Prefira

```
strings.Builder
buffers reutilizáveis
estruturas pré-alocadas
```

Também limite:

```
particle count
update rate
render area
```

---

# 71. Número de partículas

Uma cena pequena pode usar:

```
20–50 partículas
```

Uma cena média:

```
50–150
```

Uma cena mais complexa:

```
150–500+
```

Mas a quantidade ideal depende do terminal, biblioteca e complexidade do render.

Qualidade visual importa mais que quantidade.

---

# 72. Animação adaptativa

Quando o terminal é pequeno:

```
few particles
simple sprites
low FPS
```

Quando é grande:

```
more particles
larger scene
more effects
higher FPS
```

Exemplo:

```
if width < 80 {
    particleCount = 20
} else {
    particleCount = 80
}
```

---

# 73. Estados de animação

Uma cena pode possuir:

```
IDLE
STARTING
ACTIVE
ENDING
```

Exemplo:

```
STARTING
   ↓
FADE IN
   ↓
IDLE
   ↓
FADE OUT
   ↓
STOP
```

Isso facilita entrar e sair de uma animação sem cortes bruscos.

---

# 74. Sistema de ciclo

Para uma animação ambiental:

```
0s       → começa
5s       → ambiente estabiliza
20s      → pequenas variações
30s      → evento
40s      → volta ao estado inicial
```

Não precisa repetir exatamente o mesmo frame.

Use:

```
random seed
phase offsets
event timers
```

para deixar os ciclos menos previsíveis.

---

# 75. Eventos ambientais

Exemplos:

```
COMET
FIRE_CRACKLE
CAT_WAKE
FIREFLY_FLASH
LEAF_FALL
WIND_GUST
SMOKE_PUFF
STAR_TWINKLE
```

Um scheduler simples:

```
nextEventAt
```

Quando:

```
time >= nextEventAt
```

dispara o evento e agenda outro.

---

# 76. Combinação de técnicas

Uma cena realmente interessante pode usar:

```
Unicode box drawing
+
block characters
+
Braille
+
particles
+
easing
+
parallax
+
noise
+
frame animation
+
delta time
```

Não é necessário utilizar tudo ao mesmo tempo.

A combinação deve servir à cena.

---

# 77. Checklist para criar uma animação

Antes de implementar:

```
[ ] Qual é o objeto?
[ ] Qual é o tamanho?
[ ] Qual é o FPS?
[ ] Quantos frames?
[ ] O movimento é contínuo ou por frames?
[ ] Existe física?
[ ] Existem partículas?
[ ] Precisa de randomização?
[ ] Funciona em terminal pequeno?
[ ] Unicode é necessário?
[ ] Existe fallback ASCII?
[ ] A animação pode ser pausada?
[ ] O custo de renderização é aceitável?
```

---

# 78. Fallback ASCII

Se quiser máxima compatibilidade:

Unicode:

```
┌──────┐
│  ●   │
└──────┘
```

ASCII:

```
+------+
|  O   |
+------+
```

Bloco:

```
████
```

Fallback:

```
####
```

Braille:

```
⣿⣷⣶
```

Fallback:

```
###
```

---

# 79. Biblioteca mental de caracteres

Para memorizar rapidamente:

### Estrutura

```
┌ ┐ └ ┘ ├ ┤ ┬ ┴ ┼
```

### Linhas

```
─ │ ═ ║
```

### Blocos

```
█ ▓ ▒ ░
```

### Meio-blocos

```
▀ ▄ ▌ ▐
```

### Pontos

```
. · •
```

### Círculos

```
○ ● ◦ ◉
```

### Estrelas

```
★ ☆ ✦ ✧
```

### Geometria

```
◆ ◇ ■ □ ▲ △
```

### ASCII básico

```
# @ % * + = - . : ,
/ \ | _ ^ ~
```

---

# 80. Princípio final

Uma boa animação de terminal não depende de centenas de caracteres.

Ela depende principalmente de:

```
COMPOSIÇÃO
MOVIMENTO
TEMPO
CAMADAS
VARIAÇÃO
```

Uma cena simples:

```
        ·       *
   *          ·

       /\ 
      /██\
     /████\

        🔥
       ░▒▓
       ▒█▒

           ✦
```

pode parecer muito mais viva quando:

```
estrelas piscam
↓
fogo muda de forma
↓
fumaça sobe
↓
vagalumes se movem
↓
folhas caem
↓
um cometa atravessa o céu
```

Tudo isso pode ser feito sem vídeo, sem imagens externas e sem renderização gráfica tradicional.

---

# Referência rápida

```
BOX
┌ ┐ └ ┘ ├ ┤ ┬ ┴ ┼

LINES
─ │ ═ ║

BLOCKS
█ ▓ ▒ ░

HALF BLOCKS
▀ ▄ ▌ ▐

DOTS
. · • ◦

CIRCLES
○ ● ◉

STARS
★ ☆ ✦ ✧

DIAMONDS
◆ ◇

ARROWS
← → ↑ ↓ ↖ ↗ ↘ ↙

ASCII
@ # % & * + = - . , : ; ' / \ | _ ^ ~

BRAILLE
⠁ ⠂ ⠄ ⡀ ⣿

SPINNERS
⠋ ⠙ ⠹ ⠸ ⠼ ⠴ ⠦ ⠧ ⠇ ⠏

DENSITY
█▓▒░.
```

---

## Ideia central

Para criar uma TUI visualmente sofisticada:

```
                    ┌───────────────┐
                    │     UI        │
                    └───────┬───────┘
                            │
                     ┌──────▼──────┐
                     │   Scene     │
                     └──────┬──────┘
                            │
          ┌─────────────────┼─────────────────┐
          │                 │                 │
      Background        Entities          Particles
          │                 │                 │
      Stars/sky        Cat/tree/fire       smoke/fireflies
          │                 │                 │
          └─────────────────┼─────────────────┘
                            │
                       Update(dt)
                            │
                       Render Frame
                            │
                         Terminal
```

**A regra mais importante:** faça a animação parecer viva através de pequenas mudanças independentes, em vez de mover tudo simultaneamente.

Isso produz um resultado muito mais natural e é particularmente adequado para uma área de `idle/relax` em uma TUI.