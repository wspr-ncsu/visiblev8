var say_hi = print.bind(undefined, "hi!");

for (let i = 0; i < 10; ++i) {
  say_hi();
}

