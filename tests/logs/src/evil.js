var prefix = '(function(){print("';
var suffix = '");})()';

eval(prefix + "Yahoo!" + suffix); 

eval(prefix + read("/testsrc/fred.txt") + suffix);

(function() {
  this.eval(prefix + "Shazaam!" + suffix); 
})();

eval('eval("print(\\"eval-ception!\\")")');

