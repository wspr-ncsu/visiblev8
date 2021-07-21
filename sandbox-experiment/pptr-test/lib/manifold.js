// manifold: the "exhaust-manifold" of the VV8 engine---listening for log socket connections, then ingesting/annotating log streams
const net = require('net');

const VV8_LOG_SERVER_DEFAULT_PORT = 5580;
const VV8_LOG_SERVER_DEFAULT_HOST = 'localhost';


class VV8LogServer {
    constructor(/** @type number */ port, /** @type string? */ host) {
        this._port = port || VV8_LOG_SERVER_DEFAULT_PORT;
        this._host = host || VV8_LOG_SERVER_DEFAULT_HOST;
        this._topErrorHandler = null;
        this._connHandler = null;
        this._server = net.createServer((socket) => this._onConnect(socket));
        this._server.on('error', (err) => this._onError(err));
    }

    // Public API
    //-----------

    // Start listening for connections and resolve as soon as the 'listening' event is received (or reject on error)
    startListening(/** @type Function */ connHandler) {
        if (this._connHandler) {
            throw new Error("already started");
        }
        this._connHandler = connHandler;
        
        return new Promise((resolve, reject) => {
            const popErrorHandler = this._pushErrorHandler(reject);
            this._server.listen(this._port, this._host, 5, () => {
                popErrorHandler();
                resolve();
            });
        });
    }

    // Close the server and resolve as soon as all connections are closed
    stopListening() {
        return new Promise((resolve, reject) => {
            this._server.close((err) => {
                if (err) {
                    reject(err);
                } else {
                    resolve();
                }
            });
        });
    }


    // Private/Internal
    //-----------------

    // Push a new error-handler (typically a promise-reject thunk) onto the error-handler stack
    // (_onError pops from this stack, if not empty, and calls the handler with the given error)
    // returns a callable for removing this handler from the stack (or NOP if already gone)
    _pushErrorHandler(/** @type Function */ cb) {
        const parent = this;
        const link = { 
            cb,
            next: this._topErrorHandler,
            prev: null,
            dead: false,
            pop() {
                if (this.dead) return;
                this.dead = true;

                if (this.next) {
                    this.next.prev = this.prev;
                }
                
                if (this.prev) {
                    this.prev.next = this.next;
                } else {
                    parent._topErrorHandler = this.next;
                }
            }
        };
        if (link.next) {
            link.next.prev = link;
        }
        this._topErrorHandler = link;
        return () => link.pop();
    }

    // On-error handler (takes errors, passed to top error handler [if any])
    _onError(/** @type Error */ err) {
        const handler = this._topErrorHandler;
        if (handler) {
            handler.pop();
            handler.cb(err);
        } else {
            console.error('manifold.Server: unhandled error event: ', err);
        }
    }

    _onConnect(/** @type net.Socket */ socket) {
        Promise.resolve(this._connHandler(socket)).catch((err) => console.error(err));
    }
}

module.exports = {
    VV8_LOG_SERVER_DEFAULT_PORT,
    VV8_LOG_SERVER_DEFAULT_HOST,
    VV8LogServer,
};