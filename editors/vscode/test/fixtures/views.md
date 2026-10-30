# Tsuzuri preview demo

Open this file and run **Markdown: Open Preview to the Side**. See [[Project plan]] and ![[Meeting notes]].

```board
## Not started
- Write the README
## In progress
- [ ] Ship the VS Code extension
  Preview renderers and slash menu
## Done
- Release **v0.1**
## Blocked
- Marketplace icon
```

```calendar
month: 2026-10
2026-10-03: Release day
2026-10-13: Sponsor call
2026-10-13: Write changelog
2026-10-28: Demo
```

```timeline
Design: 2026-10-01 -> 2026-10-06
Build: 2026-10-05 -> 2026-10-18
Publish: 2026-10-19 -> 2026-10-20
```

```chart
type: bar
title: Notes per month
Jan: 12
Feb: 20
Mar: 7
Apr: 15
```

```chart
type: line
title: Stars
Week 1: 4
Week 2: 11
Week 3: 18
Week 4: 29
```

```chart
type: donut
title: Time spent
Writing: 50
Coding: 35
Email: 15
```

```chart
type: hbar
Vim: 40
Emacs: 22
Nano: 9
```

```form
title: Feedback
description: Tell us what you think
?* Your name
= Jai
? Favourite view (choice): Board | Calendar | Timeline
= Calendar
? Platforms (multi): Linux | macOS | Windows
= Linux, macOS
```

```flow
start([Start]) --> check{Is it working?}
check -->|yes| done([Ship it])
check -->|no| fix[Fix it]
fix --> check
```

```math
E = mc^2
\sum_{i=1}^{n} i = \frac{n(n+1)}{2}
```

~~~columns
### Left
Some notes on the left.
+++
### Right
- And a list
- on the right
~~~

```go
// Ordinary code blocks are untouched.
fmt.Println("hi")
```
