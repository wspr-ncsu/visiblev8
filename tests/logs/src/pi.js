var Pi = 0;
var n = 1;
for (var i = 0; i < 10000; ++i) {
	Pi += (4 / n);
	n += 2;
	Pi -= (4/n);
	n += 2;
	if (i % 100 == 0) print(Pi);
}

