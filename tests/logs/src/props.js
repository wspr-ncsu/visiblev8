var global = (function() { return this; })();

global.foo = 42;
print(global.foo);

for (var i = 0; i <= 1337; ++i) {
  Reflect.set(global, 'bar', i);
  print(Reflect.get(global, 'bar'));
}

