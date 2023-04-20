const a = [1,2,3,4,5];
const b = { 'a': 1, 'b': 2, 'c': 3, 'd': 4, 'e': 5 };
function A() {
    this.a = 1;
}
A.prototype.doSomething = function () {
    this.a += 1;
}
const c = new A();
print(a,b,c)