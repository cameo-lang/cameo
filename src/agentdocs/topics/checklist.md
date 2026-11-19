# Before you finish

1. `cameo check script.cameo` reports nothing.
2. The script runs and prints what was asked for.
3. No value is expected to change behind a variable: every change is an
   assignment.
4. Independent work is started with `async` and collected with `await`;
   everything else is plain calls.

An error of `cameo check`, and a refusal at run time, ends with a link,
`https://15b.it/<code>`, to a page that explains the code and what to
write instead. Read it rather than guessing. `cameo
--ignore-mutation-errors script.cameo` runs a script that calls Ruby's
mutators, `push` returning its result on a copy, to see how far it gets;
the script is still to be fixed.
