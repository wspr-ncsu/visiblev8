'use strict';

// The maximum is exclusive and the minimum is inclusive
const getRandomInt = (min, max) => {
    min = Math.ceil(min)
    max = Math.floor(max)
    return Math.floor(Math.random() * (max - min)) + min
};

const popRandomElement = (array) => {
    const ix = getRandomInt(0, array.length);
    const el = array[ix];
    array.splice(ix, 1);
    return el;
};

const closeOtherPages = async (browser, page) => {
    const allPages = await browser.pages()
    const pi = allPages.indexOf(page)
    if (pi < 0) {
        throw Error('no such page in browser')
    }
    allPages.splice(pi, 1)
    return Promise.all(allPages.map((p) => p.close()))
};

const timeoutIn = (ms) => new Promise((resolve, _) => { setTimeout(resolve, ms) });

module.exports = {
    popRandomElement,
    closeOtherPages,
    timeoutIn,
};


