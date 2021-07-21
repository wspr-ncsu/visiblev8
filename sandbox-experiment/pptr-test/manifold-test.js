const fs = require('fs');
const net = require('net');
const path = require('path');

const am = require('am');

const { VV8LogServer } = require('./lib/manifold');


const perSocketFileLogger = (/** @type string? */ rootDirectory) => {
    rootDirectory = rootDirectory || '.'
    let logCounter = 0;
    const nextLogFileName = (socket) => {
        return path.join(rootDirectory, `vv8-stream${++logCounter}-port${socket.remotePort}.log`)
    };

    return (/** @type net.Socket */ socket) => {
        const log = fs.createWriteStream(nextLogFileName(socket));
        socket.pipe(log);
    };
};

am(async () => {
    const server = new VV8LogServer(5580);
    try {
        await server.startListening(perSocketFileLogger());
    } catch (err) {
        console.error("IT GO BOOM:", err);
    }
    console.log('LISTENING');

    process.addListener('SIGINT', () => {
        server.stopListening().then(() => {
            console.log('STOPPED LISTENING');
        }).catch((err) => {
            console.error('ERROR SHUTTING DOWN:', err);
        });
    });
});