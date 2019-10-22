function Blarg(x) {
  this.x = x;
}

Blarg.prototype.say = print;

Blarg.prototype.foo = function foo() {
  this.say("foo(" + this['x'] + ")");
}


var bar = new Blarg(42);
bar.foo();
print(bar['nope']);

var flup = bar.foo.bind(bar);
flup();


