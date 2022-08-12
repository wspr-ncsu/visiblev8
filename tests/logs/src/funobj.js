function baloo(x) {
  print("Baloo say " + x);
}

baloo.call(null, "hello");

var arrrrr = function(x) {
  print("Yo-ho-ho and a bottle of " + x);
};

arrrrr.call(null, "gum");

function galump() {
  return function(x) {
    print("In Soviet Russia, " + x + " codes you!");
  };
}

galump().call(null, "JavaScript");

