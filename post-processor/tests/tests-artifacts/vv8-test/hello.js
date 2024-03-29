// let msg = "Hello World!";
// const PI = 3.14; 
 
// function addNumbers(a, b){
//     return a + b;
// }

(function () {
    console.log(navigator.userAgent)
    de = function() {
        for (let i = 1; i < 10; i++) {
            console.log(navigator.userAgent)
        }
    }
    console.trace();
    fetch("https://www.baidu.com", { mode: 'no-cors' })
    de()
    console.log("do it again")
    de()

})();