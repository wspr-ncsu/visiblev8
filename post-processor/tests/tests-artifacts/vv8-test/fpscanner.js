const UNKNOWN = 'unknown';
const ERROR = 'error';



// #### Code for Browser Attributes

const DEFAULT_ATTRIBUTES = {
    // plugins: false,
    // mimeTypes: false,
    // userAgent: false,
    // platform: false,
    isInCognito: true,
    // webDriver: false,
    // webDriverValue: false,
    languages: false,
    screen: false,
    touchScreen: false,
    videoCard: false,
    // multimediaDevices: true,
    color_gamut: false,
    speech_synthesis: true,
    // productSub: false,
    // navigatorPrototype: false,
    // etsl: false,
    // screenDesc: false,
    phantomJS: false,
    nightmareJS: false,
    selenium: false,
    // errorsGenerated: false,
    // resOverflow: false,
    accelerometerUsed: true,
    screenMediaQuery: false,
    hasChrome: false,
    hasYandex: false,
    hasOpera: false,
    hasMaxthon: false,
    hasFirefox: false,
    hasSilk: false,
    math: false,
    webglContext:true,
    // detailChrome: false,
    // permissions: true,
    // permissions: true,
    // iframeChrome: false,
    // debugTool: false,
    // battery: false,
    deviceMemory: false,
    // tpCanvas: true,
    sequentum: false,
    audio: true,
    systemColors:false,
    // audioCodecs: false,
    // videoCodecs: false
};

const defaultAttributeToFunction = {
    userAgent: () => {
        return navigator.userAgent;
    },
    math: () => {
        const mathFp = getMathFingerprint()
        return md5(JSON.stringify(mathFp))
    },
    audio: () => {
        return new Promise(resolve => {
            audioFp().then(audio_hash => {
                resolve(audio_hash)
            })
        })
    },
    systemColors: () => {
        var sys_colors = getSystemColors()
        return sys_colors
    },
    plugins: () => {
        const pluginsRes = [];
        for (let i = 0; i < navigator.plugins.length; i++) {
            const plugin = navigator.plugins[i];
            const pluginStr = [plugin.name, plugin.description, plugin.filename, plugin.version].join("::");
            let mimeTypes = [];
            Object.keys(plugin).forEach((mt) => {
                mimeTypes.push([plugin[mt].type, plugin[mt].suffixes, plugin[mt].description].join("~"));
            });
            mimeTypes = mimeTypes.join(",");
            pluginsRes.push(pluginStr + "__" + mimeTypes);
        }
        return pluginsRes;
    },
    webglContext: () => {
        return new Promise((resolve)=> {
            Promise.all([
                getWebgl('webgl'),
                getWebgl('webgl2')
            ]).then(response => {
                const webgl = response[0]
                const webgl2 = response[1]
                resolve(md5(JSON.stringify(webgl) + JSON.stringify(webgl2)))
            })
        })
        
    },
    mimeTypes: () => {
        const mimeTypes = [];
        for (let i = 0; i < navigator.mimeTypes.length; i++) {
            let mt = navigator.mimeTypes[i];
            mimeTypes.push([mt.description, mt.type, mt.suffixes].join("~~"));
        }
        return mimeTypes;
    },
    platform: () => {
        if (navigator.platform) {
            return navigator.platform;
        }
        return UNKNOWN;
    },
    languages: () => {
        if (navigator.languages) {
            return navigator.languages;
        }
        return UNKNOWN;
    },
    isInCognito: () => {
        return new Promise((resolve)=> {
            detectIncognito().then(info => {
                if (info.isPrivate) {
                    resolve("Yes")
                } else {
                    resolve("Maybe Not")
                }
            })
        })
    },
    screen: () => {
        return {
            // wInnerHeight: window.innerHeight,
            // wOuterHeight: window.outerHeight,
            // wOuterWidth: window.outerWidth,
            // wInnerWidth: window.innerWidth,
            // wScreenX: window.screenX,
            // wPageXOffset: window.pageXOffset,
            // wPageYOffset: window.pageYOffset,
            // cWidth: document.body.clientWidth,
            // cHeight: document.body.clientHeight,
            sWidth: screen.width,
            sHeight: screen.height,
            sAvailWidth: screen.availWidth,
            sAvailHeight: screen.availHeight,
            sColorDepth: screen.colorDepth,
            sPixelDepth: screen.pixelDepth,
            wDevicePixelRatio: window.devicePixelRatio
        };
    },
    touchScreen: () => {
        let maxTouchPoints = 0;
        let touchEvent = false;
        if (typeof navigator.maxTouchPoints !== "undefined") {
            maxTouchPoints = navigator.maxTouchPoints;
        } else if (typeof navigator.msMaxTouchPoints !== "undefined") {
            maxTouchPoints = navigator.msMaxTouchPoints;
        }
        try {
            document.createEvent("TouchEvent");
            touchEvent = true;
        } catch (_) {
        }

        const touchStart = "ontouchstart" in window;
        return [maxTouchPoints, touchEvent, touchStart];
    },
    videoCard: () => {
        try {
            const canvas = document.createElement('canvas');
            const ctx = canvas.getContext("webgl") || canvas.getContext("experimental-webgl");
            let webGLVendor, webGLRenderer;
            if (ctx.getSupportedExtensions().indexOf("WEBGL_debug_renderer_info") >= 0) {
                webGLVendor = ctx.getParameter(ctx.getExtension('WEBGL_debug_renderer_info').UNMASKED_VENDOR_WEBGL);
                webGLRenderer = ctx.getParameter(ctx.getExtension('WEBGL_debug_renderer_info').UNMASKED_RENDERER_WEBGL);
            } else {
                webGLVendor = "Not supported";
                webGLRenderer = "Not supported";
            }
            return [webGLVendor, webGLRenderer];
        } catch (e) {
            return "Not supported;;;Not supported";
        }
    },
    multimediaDevices: () => {
        return new Promise((resolve) => {
            const deviceToCount = {
                "audiooutput": 0,
                "audioinput": 0,
                "videoinput": 0
            };

            if (navigator.mediaDevices && navigator.mediaDevices.enumerateDevices
                && navigator.mediaDevices.enumerateDevices.name !== "bound reportBlock") {
                // bound reportBlock occurs with Brave
                navigator.mediaDevices.enumerateDevices().then((devices) => {
                    if (typeof devices !== "undefined") {
                    let name;
                    for (let i = 0; i < devices.length; i++) {
                        name = [devices[i].kind];
                        deviceToCount[name] = deviceToCount[name] + 1;
                    }
                    resolve({
                        speakers: deviceToCount.audiooutput,
                        micros: deviceToCount.audioinput,
                        webcams: deviceToCount.videoinput
                    });
                    } else {
                        resolve({
                            speakers: 0,
                            micros: 0,
                            webcams: 0
                        });
                    }

                });
            } else if (navigator.mediaDevices && navigator.mediaDevices.enumerateDevices
                && navigator.mediaDevices.enumerateDevices.name === "bound reportBlock") {
                resolve({
                    'devicesBlockedByBrave': true
                });
            } else {
                resolve({
                    speakers: 0,
                    micros: 0,
                    webcams: 0
                });
            }
        });
    },
    color_gamut: () => {
        const color_gamuts = ['rec2020', 'p3', 'srgb']
        for (var i in color_gamuts) {
            if (matchMedia(`(color-gamut: ${color_gamuts[i]})`).matches) {
                return color_gamuts[i]
            }
        }
        return 'Undefined'
    },
    speech_synthesis: () => {
        return new Promise(
            function (resolve, reject) {
                let synth = window.speechSynthesis;
                let id;
    
                id = setInterval(() => {
                    if (synth.getVoices().length !== 0) {
                        // console.log(synth.getVoices())
                        resolve(md5(JSON.stringify(synth.getVoices())));
                        clearInterval(id);
                    }
                }, 10);
            }
        )
        
    },
    productSub: () => {

        return navigator.productSub;
    },
    navigatorPrototype: () => {
        let obj = window.navigator;
        const protoNavigator = [];
        do Object.getOwnPropertyNames(obj).forEach((name) => {
            protoNavigator.push(name);
        });
        while (obj = Object.getPrototypeOf(obj));

        let res;
        const finalProto = [];
        protoNavigator.forEach((prop) => {
            const objDesc = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(navigator), prop);
            if (objDesc !== undefined) {
                if (objDesc.value !== undefined) {
                    res = objDesc.value.toString();
                } else if (objDesc.get !== undefined) {
                    res = objDesc.get.toString();
                }
            }
            else {
                res = "";
            }
            finalProto.push(prop + "~~~" + res);
        });
        return finalProto;
    },
    etsl: () => {
        return eval.toString().length;
    },
    screenDesc: () => {
        try {
            return Object.getOwnPropertyDescriptor(Object.getPrototypeOf(screen), "width").get.toString();
        } catch (e) {
            return ERROR;
        }
    },
    nightmareJS: () => {
        return !!window.__nightmare;
    },
    phantomJS: () => {
        return [
            'callPhantom' in window,
            '_phantom' in window,
            'phantom' in window
        ];
    },
    selenium: () => {
        return [
            'webdriver' in window,
            '_Selenium_IDE_Recorder' in window,
            'callSelenium' in window,
            '_selenium' in window,
            '__webdriver_script_fn' in document,
            '__driver_evaluate' in document,
            '__webdriver_evaluate' in document,
            '__selenium_evaluate' in document,
            '__fxdriver_evaluate' in document,
            '__driver_unwrapped' in document,
            '__webdriver_unwrapped' in document,
            '__selenium_unwrapped' in document,
            '__fxdriver_unwrapped' in document,
            '__webdriver_script_func' in document,
            document.documentElement.getAttribute("selenium") !== null,
            document.documentElement.getAttribute("webdriver") !== null,
            document.documentElement.getAttribute("driver") !== null
        ];
    },
    webDriver: () => {
        return 'webdriver' in navigator;
    },
    webDriverValue: () => {
        return navigator.webdriver;
    },
    errorsGenerated: () => {
        const errors = [];
        try {
            azeaze + 3;
        } catch (e) {
            errors.push(e.message);
            errors.push(e.fileName);
            errors.push(e.lineNumber);
            errors.push(e.description);
            errors.push(e.number);
            errors.push(e.columnNumber);
            try {
                errors.push(e.toSource().toString());
            } catch (e) {
                errors.push(undefined);
            }
        }

        try {
            new WebSocket('itsgonnafail');
        } catch (e) {
            errors.push(e.message);
        }
        return errors;
    },
    resOverflow: () => {
        let depth = 0;
        let errorMessage = '';
        let errorName = '';
        let errorStacklength = 0;

        function iWillBetrayYouWithMyLongName() {
            try {
                depth++;
                iWillBetrayYouWithMyLongName();
            } catch (e) {
                errorMessage = e.message;
                errorName = e.name;
                errorStacklength = e.stack.toString().length;
            }
        }

        iWillBetrayYouWithMyLongName();
        return {
            depth: depth,
            errorMessage: errorMessage,
            errorName: errorName,
            errorStacklength: errorStacklength
        }

    },
    accelerometerUsed: () => {
        return new Promise((resolve) => {
            window.ondevicemotion = event => {
                if (event.accelerationIncludingGravity.x !== null) {
                    return resolve(true);
                }
            };

            setTimeout(() => {
                return resolve(false);
            }, 300);
        });
    },
    screenMediaQuery: () => {
        return window.matchMedia('(min-width: ' + (window.innerWidth - 1) + 'px)').matches;
    },
    hasChrome: () => {
        return !!window.chrome;
    },
    hasYandex: () => {
        return !!window.yandex;
    },
    hasOpera: () => {
        return !!window.opera;
    },
    hasMaxthon: () => {
        return window.mxZoomFactor;
    },
    hasSilk: () => {
        return !!window.com_amazon_cloud9_immersion
    },
    hasFirefox: () => {
        return typeof(InstallTrigger) !== "undefined"
    },
    detailChrome: () => {
        if (!window.chrome) return UNKNOWN;

        const res = {};

        try{
            ["webstore", "runtime", "app", "csi", "loadTimes"].forEach((property) => {
                res[property] = window.chrome[property].constructor.toString().length;
            });
        } catch (e) {
            res.properties = UNKNOWN;
        }

        try {
            window.chrome.runtime.connect('');
        } catch (e) {
            res.connect = e.message.length;
        }
        try {
            window.chrome.runtime.sendMessage();
        } catch (e) {
            res.sendMessage = e.message.length;
        }

        return JSON.stringify(res);
    },
    iframeChrome: () => {
        const iframe = document.createElement('iframe');
        iframe.srcdoc = 'blank page';
        document.body.appendChild(iframe);

        const result = typeof iframe.contentWindow.chrome;
        iframe.remove();

        return result;
    },
    debugTool: () => {
        let cpt = 0;
        const regexp = /./;
        regexp.toString = () => {
            cpt++;
            return 'spooky';
        };
        console.debug(regexp);
        return cpt > 1;
    },
    battery: () => {
        return 'getBattery' in window.navigator;
    },
    deviceMemory: () => {
        return navigator.deviceMemory || 0;
    },
    tpCanvas: () => {
        return new Promise((resolve) => {
            try {
                const img = new Image();
                const canvasCtx = document.createElement('canvas').getContext('2d');
                img.onload = () => {
                    canvasCtx.drawImage(img, 0, 0);
                    resolve(canvasCtx.getImageData(0, 0, 1, 1).data);
                };

                img.onerror = () => {
                    resolve(ERROR);
                };
                img.src = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVQYV2NgAAIAAAUAAarVyFEAAAAASUVORK5CYII=';
            } catch (e) {
                resolve(ERROR);
            }
        });
    },
    sequentum: () => {
        return window.external && window.external.toString && window.external.toString().indexOf('Sequentum') > -1;
    },
    audioCodecs: () => {
        const audioElt = document.createElement("audio");

        if (audioElt.canPlayType) {
            return {
                ogg: audioElt.canPlayType('audio/ogg; codecs="vorbis"'),
                mp3: audioElt.canPlayType('audio/mpeg;'),
                wav: audioElt.canPlayType('audio/wav; codecs="1"'),
                m4a: audioElt.canPlayType('audio/x-m4a;'),
                aac: audioElt.canPlayType('audio/aac;'),
            }
        }
        return {
            ogg: UNKNOWN,
            mp3: UNKNOWN,
            wav: UNKNOWN,
            m4a: UNKNOWN,
            aac: UNKNOWN
        };
    },
    videoCodecs: () => {
        const videoElt = document.createElement("video");

        if (videoElt.canPlayType) {
            return {
                ogg: videoElt.canPlayType('video/ogg; codecs="theora"'),
                h264: videoElt.canPlayType('video/mp4; codecs="avc1.42E01E"'),
                webm: videoElt.canPlayType('video/webm; codecs="vp8, vorbis"'),
            }
        }
        return {
            ogg: UNKNOWN,
            h264: UNKNOWN,
            webm: UNKNOWN,
        }
    },
    
    permissions: () => {
        return new Promise((resolve) => {
            const promises = []
            const results = []
            // safari does not support permissions
            if (typeof navigator.permissions === "undefined") {
                return resolve(results)
            }
            const permissions = {}
            const temp = []
            const arr = ['speaker','geolocation','microphone','camera','device-info','midi','background-sync','bluetooth','persistent-storage','ambient-light-sensor','accelerometer','gyroscope','magnetometer','clipboard','accessibility-events','clipboard-read','clipboard-write','payment-hander']
            arr.forEach((permission_class) => {
                promises.push(new Promise((resolve)=>{
                    _permissions(permission_class).then((val) => {
                        results.push(val)
                        return resolve()
                    }).catch((e)=> {
                        console.log('query not allowed')
                    })
                }))
            })

            return Promise.all(promises).then(() =>{
                return resolve(results);
            });
        })
        
        
    },
};

const _permissions = function (classType) {
    return new Promise((resolve) => {
        navigator.permissions.query({name: classType}).then((val) => {
            resolve({
                name: classType,
                state: val.state,
            })
        }).catch((e)=> {
            resolve({
                name: classType,
                state: 'query not allowed',
            })
        })
    })
}

const addCustomFunction = function (name, isAsync, f) {
    DEFAULT_ATTRIBUTES[name] = isAsync;
    defaultAttributeToFunction[name] = f;
};

function loadChartbeat() {
    var e = document.createElement('script');
    var n = document.getElementsByTagName('script')[0];
    console.log
    e.type = 'text/javascript';
    e.async = true;
    // e.src = '//static.chartbeat.com/js/chartbeat.js';
    e.src = '//cdn.yektanet.com/fp/fingerprint.js?v=umd';
    
    n.parentNode.insertBefore(e, n);
}

function loadHello() {
    var e = document.createElement('script');
    var n = document.getElementsByTagName('script')[0];
    console.log
    e.type = 'text/javascript';
    // e.async = true;
    e.src = '/hello.js';
    n.parentNode.insertBefore(e, n);
}

const generateFingerprint = function () {
    // const a = performance.now()
    return new Promise((resolve) => {
        const promises = [];
        // console.log(addNumbers(1,2))
        loadChartbeat();
        loadHello();
        


        const fingerprint = {};
        Object.keys(DEFAULT_ATTRIBUTES).forEach((attribute) => {
            fingerprint[attribute] = {};
            
            if (DEFAULT_ATTRIBUTES[attribute]) {
                promises.push(new Promise((resolve) => {
                    defaultAttributeToFunction[attribute]().then((val) => {
                        fingerprint[attribute] = val;
                        return resolve();
                    }).catch((e) => {
                        fingerprint[attribute] = {
                            error: true,
                            message: e.toString()
                        };
                        return resolve();
                    })
                }));
            } else {
                try {
                    fingerprint[attribute] = defaultAttributeToFunction[attribute]();
                } catch (e) {
                    fingerprint[attribute] = {
                        error: true,
                        message: e.toString()
                    };
                }
            }
        });
        return Promise.all(promises).then(() => {
            // const b = performance.now()
            // split = b - a
            // document.getElementById("time").innerText="time: "+split.toString() + " ms"
            // console.log(fingerprint['permissions'])
            return resolve(fingerprint);
        });
    });
};


// #### Code for MD5 algorithm
function md5cycle(x, k) {
    var a = x[0], b = x[1], c = x[2], d = x[3];

    a = ff(a, b, c, d, k[0], 7, -680876936);
    d = ff(d, a, b, c, k[1], 12, -389564586);
    c = ff(c, d, a, b, k[2], 17, 606105819);
    b = ff(b, c, d, a, k[3], 22, -1044525330);
    a = ff(a, b, c, d, k[4], 7, -176418897);
    d = ff(d, a, b, c, k[5], 12, 1200080426);
    c = ff(c, d, a, b, k[6], 17, -1473231341);
    b = ff(b, c, d, a, k[7], 22, -45705983);
    a = ff(a, b, c, d, k[8], 7, 1770035416);
    d = ff(d, a, b, c, k[9], 12, -1958414417);
    c = ff(c, d, a, b, k[10], 17, -42063);
    b = ff(b, c, d, a, k[11], 22, -1990404162);
    a = ff(a, b, c, d, k[12], 7, 1804603682);
    d = ff(d, a, b, c, k[13], 12, -40341101);
    c = ff(c, d, a, b, k[14], 17, -1502002290);
    b = ff(b, c, d, a, k[15], 22, 1236535329);

    a = gg(a, b, c, d, k[1], 5, -165796510);
    d = gg(d, a, b, c, k[6], 9, -1069501632);
    c = gg(c, d, a, b, k[11], 14, 643717713);
    b = gg(b, c, d, a, k[0], 20, -373897302);
    a = gg(a, b, c, d, k[5], 5, -701558691);
    d = gg(d, a, b, c, k[10], 9, 38016083);
    c = gg(c, d, a, b, k[15], 14, -660478335);
    b = gg(b, c, d, a, k[4], 20, -405537848);
    a = gg(a, b, c, d, k[9], 5, 568446438);
    d = gg(d, a, b, c, k[14], 9, -1019803690);
    c = gg(c, d, a, b, k[3], 14, -187363961);
    b = gg(b, c, d, a, k[8], 20, 1163531501);
    a = gg(a, b, c, d, k[13], 5, -1444681467);
    d = gg(d, a, b, c, k[2], 9, -51403784);
    c = gg(c, d, a, b, k[7], 14, 1735328473);
    b = gg(b, c, d, a, k[12], 20, -1926607734);

    a = hh(a, b, c, d, k[5], 4, -378558);
    d = hh(d, a, b, c, k[8], 11, -2022574463);
    c = hh(c, d, a, b, k[11], 16, 1839030562);
    b = hh(b, c, d, a, k[14], 23, -35309556);
    a = hh(a, b, c, d, k[1], 4, -1530992060);
    d = hh(d, a, b, c, k[4], 11, 1272893353);
    c = hh(c, d, a, b, k[7], 16, -155497632);
    b = hh(b, c, d, a, k[10], 23, -1094730640);
    a = hh(a, b, c, d, k[13], 4, 681279174);
    d = hh(d, a, b, c, k[0], 11, -358537222);
    c = hh(c, d, a, b, k[3], 16, -722521979);
    b = hh(b, c, d, a, k[6], 23, 76029189);
    a = hh(a, b, c, d, k[9], 4, -640364487);
    d = hh(d, a, b, c, k[12], 11, -421815835);
    c = hh(c, d, a, b, k[15], 16, 530742520);
    b = hh(b, c, d, a, k[2], 23, -995338651);

    a = ii(a, b, c, d, k[0], 6, -198630844);
    d = ii(d, a, b, c, k[7], 10, 1126891415);
    c = ii(c, d, a, b, k[14], 15, -1416354905);
    b = ii(b, c, d, a, k[5], 21, -57434055);
    a = ii(a, b, c, d, k[12], 6, 1700485571);
    d = ii(d, a, b, c, k[3], 10, -1894986606);
    c = ii(c, d, a, b, k[10], 15, -1051523);
    b = ii(b, c, d, a, k[1], 21, -2054922799);
    a = ii(a, b, c, d, k[8], 6, 1873313359);
    d = ii(d, a, b, c, k[15], 10, -30611744);
    c = ii(c, d, a, b, k[6], 15, -1560198380);
    b = ii(b, c, d, a, k[13], 21, 1309151649);
    a = ii(a, b, c, d, k[4], 6, -145523070);
    d = ii(d, a, b, c, k[11], 10, -1120210379);
    c = ii(c, d, a, b, k[2], 15, 718787259);
    b = ii(b, c, d, a, k[9], 21, -343485551);

    x[0] = add32(a, x[0]);
    x[1] = add32(b, x[1]);
    x[2] = add32(c, x[2]);
    x[3] = add32(d, x[3]);

}

function cmn(q, a, b, x, s, t) {
    a = add32(add32(a, q), add32(x, t));
    return add32((a << s) | (a >>> (32 - s)), b);
}

function ff(a, b, c, d, x, s, t) {
    return cmn((b & c) | ((~b) & d), a, b, x, s, t);
}

function gg(a, b, c, d, x, s, t) {
    return cmn((b & d) | (c & (~d)), a, b, x, s, t);
}

function hh(a, b, c, d, x, s, t) {
    return cmn(b ^ c ^ d, a, b, x, s, t);
}

function ii(a, b, c, d, x, s, t) {
    return cmn(c ^ (b | (~d)), a, b, x, s, t);
}

function md51(s) {
    txt = '';
    var n = s.length,
        state = [1732584193, -271733879, -1732584194, 271733878], i;
    for (i = 64; i <= s.length; i += 64) {
        md5cycle(state, md5blk(s.substring(i - 64, i)));
    }
    s = s.substring(i - 64);
    var tail = [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0];
    for (i = 0; i < s.length; i++)
        tail[i >> 2] |= s.charCodeAt(i) << ((i % 4) << 3);
    tail[i >> 2] |= 0x80 << ((i % 4) << 3);
    if (i > 55) {
        md5cycle(state, tail);
        for (i = 0; i < 16; i++) tail[i] = 0;
    }
    tail[14] = n * 8;
    md5cycle(state, tail);
    return state;
}

/* there needs to be support for Unicode here,
 * unless we pretend that we can redefine the MD-5
 * algorithm for multi-byte characters (perhaps
 * by adding every four 16-bit characters and
 * shortening the sum to 32 bits). Otherwise
 * I suggest performing MD-5 as if every character
 * was two bytes--e.g., 0040 0025 = @%--but then
 * how will an ordinary MD-5 sum be matched?
 * There is no way to standardize text to something
 * like UTF-8 before transformation; speed cost is
 * utterly prohibitive. The JavaScript standard
 * itself needs to look at this: it should start
 * providing access to strings as preformed UTF-8
 * 8-bit unsigned value arrays.
 */
function md5blk(s) { /* I figured global was faster.   */
    var md5blks = [], i; /* Andy King said do it this way. */
    for (i = 0; i < 64; i += 4) {
        md5blks[i >> 2] = s.charCodeAt(i)
            + (s.charCodeAt(i + 1) << 8)
            + (s.charCodeAt(i + 2) << 16)
            + (s.charCodeAt(i + 3) << 24);
    }
    return md5blks;
}

var hex_chr = '0123456789abcdef'.split('');

function rhex(n) {
    var s = '', j = 0;
    for (; j < 4; j++)
        s += hex_chr[(n >> (j * 8 + 4)) & 0x0F]
            + hex_chr[(n >> (j * 8)) & 0x0F];
    return s;
}

function hex(x) {
    for (var i = 0; i < x.length; i++)
        x[i] = rhex(x[i]);
    return x.join('');
}

function md5(s) {
    return hex(md51(s));
}

/* this function is much faster,
so if possible we use it. Some IEs
are the only ones I know of that
need the idiotic second function,
generated by an if clause.  */

function add32(a, b) {
    return (a + b) & 0xFFFFFFFF;
}


// #### Code for incognito

function detectIncognito() {
    return new Promise(function (resolve, reject) {
        var browserName = "Unknown";
        function __callback(isPrivate) {
            resolve({
                isPrivate: isPrivate,
                browserName: browserName,
            });
        }
        function identifyChromium() {
            var ua = navigator.userAgent;
            if (ua.match(/Chrome/)) {
                if (navigator.brave !== undefined) {
                    return "Brave";
                }
                else if (ua.match(/Edg/)) {
                    return "Edge";
                }
                else if (ua.match(/OPR/)) {
                    return "Opera";
                }
                return "Chrome";
            }
            else {
                return "Chromium";
            }
        }
        function assertEvalToString(value) {
            return value === eval.toString().length;
        }
        function isSafari() {
            var v = navigator.vendor;
            return (v !== undefined && v.indexOf("Apple") === 0 && assertEvalToString(37));
        }
        function isChrome() {
            var v = navigator.vendor;
            return (v !== undefined && v.indexOf("Google") === 0 && assertEvalToString(33));
        }
        function isFirefox() {
            return (document.documentElement !== undefined &&
                document.documentElement.style.MozAppearance !== undefined &&
                assertEvalToString(37));
        }
        function isMSIE() {
            return (navigator.msSaveBlob !== undefined && assertEvalToString(39));
        }
        /**
         * Safari (Safari for iOS & macOS)
         **/
        function macOS_safari14() {
            try {
                window.safari.pushNotification.requestPermission("https://example.com", "private", {}, function () { });
            }
            catch (e) {
                return __callback(!new RegExp("gesture").test(e));
            }
            return __callback(false);
        }
        function iOS_safari14() {
            var tripped = false;
            var iframe = document.createElement("iframe");
            iframe.style.display = "none";
            document.body.appendChild(iframe);
            iframe.contentWindow.applicationCache.addEventListener("error", function () {
                tripped = true;
                return __callback(true);
            });
            setTimeout(function () {
                if (!tripped) {
                    __callback(false);
                }
            }, 100);
        }
        function oldSafariTest() {
            var openDB = window.openDatabase;
            var storage = window.localStorage;
            try {
                openDB(null, null, null, null);
            }
            catch (e) {
                return __callback(true);
            }
            try {
                storage.setItem("test", "1");
                storage.removeItem("test");
            }
            catch (e) {
                return __callback(true);
            }
            return __callback(false);
        }
        function safariPrivateTest() {
            var w = window;
            if (navigator.maxTouchPoints !== undefined) {
                if (w.safari !== undefined && w.DeviceMotionEvent === undefined) {
                    browserName = "Safari for macOS";
                    macOS_safari14();
                }
                else if (w.DeviceMotionEvent !== undefined) {
                    browserName = "Safari for iOS";
                    iOS_safari14();
                }
                else {
                    reject(new Error("detectIncognito Could not identify this version of Safari"));
                }
            }
            else {
                browserName = "Safari";
                oldSafariTest();
            }
        }
        /**
         * Chrome
         **/
        function getQuotaLimit() {
            var w = window;
            if (w.performance !== undefined &&
                w.performance.memory !== undefined &&
                w.performance.memory.jsHeapSizeLimit !== undefined) {
                return performance.memory.jsHeapSizeLimit;
            }
            return 1073741824;
        }
        // >= 76
        function storageQuotaChromePrivateTest() {
            navigator.webkitTemporaryStorage.queryUsageAndQuota(function (usage, quota) {
                __callback(quota < getQuotaLimit());
            }, function (e) {
                reject(new Error("detectIncognito somehow failed to query storage quota: " +
                    e.message));
            });
        }
        // 50 to 75
        function oldChromePrivateTest() {
            var fs = window.webkitRequestFileSystem;
            var success = function () {
                __callback(false);
            };
            var error = function () {
                __callback(true);
            };
            fs(0, 1, success, error);
        }
        function chromePrivateTest() {
            if (Promise !== undefined && Promise.allSettled !== undefined) {
                storageQuotaChromePrivateTest();
            }
            else {
                oldChromePrivateTest();
            }
        }
        /**
         * Firefox
         **/
        function firefoxPrivateTest() {
            __callback(navigator.serviceWorker === undefined);
        }
        /**
         * MSIE
         **/
        function msiePrivateTest() {
            __callback(window.indexedDB === undefined);
        }
        function main() {
            if (isSafari()) {
                safariPrivateTest();
            }
            else if (isChrome()) {
                browserName = identifyChromium();
                chromePrivateTest();
            }
            else if (isFirefox()) {
                browserName = "Firefox";
                firefoxPrivateTest();
            }
            else if (isMSIE()) {
                browserName = "Internet Explorer";
                msiePrivateTest();
            }
            else {
                reject(new Error("detectIncognito cannot determine the browser"));
            }
        }
        main();
    });
};


// ####Code for Math fingerprinting

const M = Math // To reduce the minified code size
const fallbackFn = () => 0


function getMathFingerprint() {
    // Native operations
  const acos = M.acos || fallbackFn
  const acosh = M.acosh || fallbackFn
  const asin = M.asin || fallbackFn
  const asinh = M.asinh || fallbackFn
  const atanh = M.atanh || fallbackFn
  const atan = M.atan || fallbackFn
  const sin = M.sin || fallbackFn
  const sinh = M.sinh || fallbackFn
  const cos = M.cos || fallbackFn
  const cosh = M.cosh || fallbackFn
  const tan = M.tan || fallbackFn
  const tanh = M.tanh || fallbackFn
  const exp = M.exp || fallbackFn
  const expm1 = M.expm1 || fallbackFn
  const log1p = M.log1p || fallbackFn


  // Operation polyfills
  const powPI = (value) => M.pow(M.PI, value)
  const acoshPf = (value) => M.log(value + M.sqrt(value * value - 1))
  const asinhPf = (value) => M.log(value + M.sqrt(value * value + 1))
  const atanhPf = (value) => M.log((1 + value) / (1 - value)) / 2
  const sinhPf = (value) => M.exp(value) - 1 / M.exp(value) / 2
  const coshPf = (value) => (M.exp(value) + 1 / M.exp(value)) / 2
  const expm1Pf = (value) => M.exp(value) - 1
  const tanhPf = (value) => (M.exp(2 * value) - 1) / (M.exp(2 * value) + 1)
  const log1pPf = (value) => M.log(1 + value)

  // Note: constant values are empirical
  return {
    acos: acos(0.123124234234234242),
    acosh: acosh(1e308),
    acoshPf: acoshPf(1e154), // 1e308 will not work for polyfill
    asin: asin(0.123124234234234242),
    asinh: asinh(1),
    asinhPf: asinhPf(1),
    atanh: atanh(0.5),
    atanhPf: atanhPf(0.5),
    atan: atan(0.5),
    sin: sin(-1e300),
    sinh: sinh(1),
    sinhPf: sinhPf(1),
    cos: cos(10.000000000123),
    cosh: cosh(1),
    coshPf: coshPf(1),
    tan: tan(-1e300),
    tanh: tanh(1),
    tanhPf: tanhPf(1),
    exp: exp(1),
    expm1: expm1(1),
    expm1Pf: expm1Pf(1),
    log1p: log1p(10),
    log1pPf: log1pPf(10),
    powPI: powPI(-100),
  }
}


function calculateHash(samples) {
    let hash = 0
    for (let i = 0; i < samples.length; ++i) {
        hash += Math.abs(samples[i])
    }
    return hash
}

function audioFp() {
    return new Promise((resolve) => {
        const AudioContext = window.OfflineAudioContext || window.webkitOfflineAudioContex // Safari doesnt support OfflineAudioContext

    const context = new AudioContext(1, 5000, 44100)

    const oscillator = context.createOscillator()
    oscillator.type = "triangle"
    oscillator.frequency.value = 1000


    const compressor = context.createDynamicsCompressor()
    compressor.threshold.value = -50
    compressor.knee.value = 40
    compressor.ratio.value = 12
    compressor.reduction.value = 20
    compressor.attack.value = 0
    compressor.release.value = 0.2

    oscillator.connect(compressor)
    compressor.connect(context.destination);

    oscillator.start()

    const resumeTriesMaxCount = 3
    const runningTimeout = 1000
    var audio_hash = ''
    context.oncomplete = event => {
        // We have only one channel, so we get it by index
        const samples = event.renderedBuffer.getChannelData(0)
        const hash = md5(calculateHash(samples).toString())
        audio_hash = hash.toString()
        resolve(audio_hash)
      };
      context.startRendering()
    })
}

function makeInnerError(name) {
    const error = new Error(name)
    error.name = name
    return error
}




// #### Code for System colors

function getSystemColors() {
	/** All system colors specified by W3C, see: http://www.w3.org/TR/css3-color/#css-system */
	// var colors = new Array('ActiveBorder', 'ActiveCaption', 'AppWorkspace', 'Background', 'ButtonFace', 'ButtonHighlight', 'ButtonShadow', 'ButtonText',
	// 				 	   'CaptionText', 'GrayText', 'Highlight', 'HighlightText', 'InactiveBorder', 'InactiveCaption', 'InactiveCaptionText',
	// 				 	   'InfoBackground', 'InfoText', 'Menu', 'MenuText', 'Scrollbar', 'ThreeDDarkShadow', 'ThreeDFace', 'ThreeDHighlight',
	// 				 	   'ThreeDLightShadow', 'ThreeDShadow', 'Window', 'WindowFrame', 'WindowText');

    var colors = new Array('ActiveText', 'ButtonBorder', 'ButtonFace', 'ButtonText', 'Canvas', 'CanvasText', 'Field', 'FieldText',
					 	'GrayText', 'Highlight', 'HighlightText', 'LinkText', 'Mark', 'MarkText', 'VisitedText');

	/** Results, saved in an associative array */
	var obj = new Object();				 	   

	/** Try to create an empty DIV object */	
	try {
		var div = document.createElement('div');
		div.style.display = 'block';
		div.style.visibility = 'hidden'
		div.style.position = 'absolute';
		div.style.left = '-100px';
		div.style.top = '-100px';
		div.style.width = '5px';
		div.style.height = '5px';
		div.style.backgroundColor = '#000000';
		div.id = 'syscolor';
		document.getElementsByTagName('body')[0].appendChild(div);
	} catch (e) {
        console.log(e)
		document.write('<div id="syscolor" style="display:block; visibility:hidden"></div>');
	}
	
	try {
		if (typeof(div) == 'undefined')
			var div = document.getElementById('syscolor');
		
		/** Regular expression, detect CSS-style RGB color information like 'rgb(255,255,0)' */
		var re = /rgb\((\d{1,3}),\s*(\d{1,3}),\s*(\d{1,3})\)/;
		var c;
		
		/** Identifier is used for names in associative array obj. These match the names of coloumns in the MySQL table (system color with 'color_' prefix). */
		var identifier;
		
		for (var i = 0; i < colors.length; i++) {
			/** Set background color to one of the system colors */
			div.style.backgroundColor = colors[i];
			
			identifier = 'color_' + colors[i].toString().toLowerCase();
			
			/** Different methods of getting real RGB values */
			if (div.currentStyle) {	/** Browser that supports 'div.currentStyle' (Internet Explorer */
				c = div.currentStyle['backgroundColor'];
			} else { /** Other browsers */
				c = document.defaultView.getComputedStyle(div, null).getPropertyValue('background-color');
			}
			
			/**
			 * Check if retrieved color is not null and is not the name of the system color.
			 * When run in Internet Explorer most versions only return the name of the system color (e.g. 'ActiveBorder'),
			 * so we cannot retrieve real RGB values.
			 */
			if (c != null && c.toString().toLowerCase() != colors[i].toLowerCase()) {
				c = c.toString();
				
				/** Check if returned value is hex-formatted (e.g. #ffff00) or in 'rgb(r,g,b)' format */
				results = re.exec(c);
				
				if (results != null) {
					/** 'rgb(r,g,b)' format: Use return values and shift them bitwise to create hex-format */
					//obj[identifier] = ('#' + (parseInt(results[1]) << 16 | parseInt(results[2]) << 8 | parseInt(results[3])).toString(16)).toString();
					obj[identifier] = '#' + (parseInt(results[1])).toString(16).padStart(2,"0") + (parseInt(results[2])).toString(16).padStart(2,"0") + (parseInt(results[3])).toString(16).padStart(2,"0");

				} else {
					/** Probably hex format: Check if color information already includes a hash at the beginning. If not, add it. */
					if (c.substr(0, 1) != '#')
						c = '#' + c;
					
					/** Check if string does not contain more than 7 characters. If so, it is probably a well formated hex-coded color information. */
					if (c.length <= 7)
						obj[identifier] = c.toString();	
				}
			} else {
				/** Color information cannot be retrieved. */
				obj[identifier] = null;
			}
		}
		return md5(JSON.stringify(obj));
	} catch(e) {
		return md5(JSON.stringify(obj));
	}
}


// #### Code for WebGL contexts

/* parameter helpers */
// https://developer.mozilla.org/en-US/docs/Web/API/EXT_texture_filter_anisotropic
const getMaxAnisotropy = context => {
    try {
        const extension = (
            context.getExtension('EXT_texture_filter_anisotropic') ||
            context.getExtension('WEBKIT_EXT_texture_filter_anisotropic') ||
            context.getExtension('MOZ_EXT_texture_filter_anisotropic')
        )
        return context.getParameter(extension.MAX_TEXTURE_MAX_ANISOTROPY_EXT)
    } catch (error) {
        console.error(error)
        return undefined
    }
}

// https://developer.mozilla.org/en-US/docs/Web/API/WEBGL_draw_buffers
const getMaxDrawBuffers = context => {
    try {
        const extension = (
            context.getExtension('WEBGL_draw_buffers') ||
            context.getExtension('WEBKIT_WEBGL_draw_buffers') ||
            context.getExtension('MOZ_WEBGL_draw_buffers')
        )
        return context.getParameter(extension.MAX_DRAW_BUFFERS_WEBGL)
    } catch (error) {
        return undefined
    }
}

// https://developer.mozilla.org/en-US/docs/Web/API/WebGLShaderPrecisionFormat/precision
// https://developer.mozilla.org/en-US/docs/Web/API/WebGLShaderPrecisionFormat/rangeMax
// https://developer.mozilla.org/en-US/docs/Web/API/WebGLShaderPrecisionFormat/rangeMin
const getShaderData = (name, shader) => {
    const shaderData = {}
    try {
        for (const prop in shader) {
            const shaderPrecisionFormat = shader[prop]
            shaderData[prop] = {
                precision: shaderPrecisionFormat.precision,
                rangeMax: shaderPrecisionFormat.rangeMax,
                rangeMin: shaderPrecisionFormat.rangeMin
            }
        }
        return shaderData
    } catch (error) {
        return undefined
    }
}

// https://developer.mozilla.org/en-US/docs/Web/API/WebGLRenderingContext/getShaderPrecisionFormat
const getShaderPrecisionFormat = (context, shaderType) => {
    const props = ['LOW_FLOAT', 'MEDIUM_FLOAT', 'HIGH_FLOAT']
    const precisionFormat = {}
    try {
        props.forEach(prop => {
            precisionFormat[prop] = context.getShaderPrecisionFormat(context[shaderType], context[prop])
            return
        })
        return precisionFormat
    } catch (error) {
        return undefined
    }
}

// https://developer.mozilla.org/en-US/docs/Web/API/WEBGL_debug_renderer_info
const getUnmasked = (context, constant) => {
    try {
        const extension = context.getExtension('WEBGL_debug_renderer_info')
        const unmasked = context.getParameter(extension[constant])
        return unmasked
    } catch (error) {
        return undefined
    }
}

/* get WebGLRenderingContext or WebGL2RenderingContext */
// https://developer.mozilla.org/en-US/docs/Web/API/WebGLRenderingContext
// https://developer.mozilla.org/en-US/docs/Web/API/WebGL2RenderingContext
const getWebgl = type => {
    return new Promise(resolve => {

        // detect support
        if (type == 'webgl' && !('WebGLRenderingContext' in window)) {
            return resolve(undefined)
        } else if (type == 'webgl2' && !('WebGL2RenderingContext' in window)) {
            return resolve(undefined)
        }

        // get canvas context
        let canvas = {}
        let context = {}
        try {
            canvas = document.createElement('canvas')
            context = canvas.getContext(type) || canvas.getContext('experimental-' + type)
        } catch (error) {
            console.error(error)
            return resolve(undefined)
        }

        // get webgl2 new methods
        // https://developer.mozilla.org/en-US/docs/Web/API/WebGL2RenderingContext
        let newMethods = []

        if (type == 'webgl2') {
            try {
                const version1Props = Object.getOwnPropertyNames(WebGLRenderingContext.prototype)
                const version2Props = Object.getOwnPropertyNames(WebGL2RenderingContext.prototype)
                const version1Methods = new Set(version1Props.filter(name => typeof context[name] == 'function'))
                const version2Methods = new Set(version2Props.filter(name => typeof context[name] == 'function'))
                newMethods = [...new Set([...version2Methods].filter(method => !version1Methods.has(method)))]
            } catch (error) {
                console.error(error)
            }
        }

        // get supported extensions
        // https://developer.mozilla.org/en-US/docs/Web/API/WebGLRenderingContext/getSupportedExtensions
        // https://developer.mozilla.org/en-US/docs/Web/API/WebGL_API/Using_Extensions
        let supportedExtensions = []
        try {
            supportedExtensions = context.getSupportedExtensions()
        } catch (error) {
            console.error(error)
        }

        // get parameters
        // https://developer.mozilla.org/en-US/docs/Web/API/WebGL_API/Constants
        const version1Constants = [
            'ALIASED_LINE_WIDTH_RANGE',
            'ALIASED_POINT_SIZE_RANGE',
            'ALPHA_BITS',
            'BLUE_BITS',
            'DEPTH_BITS',
            'GREEN_BITS',
            'MAX_COMBINED_TEXTURE_IMAGE_UNITS',
            'MAX_CUBE_MAP_TEXTURE_SIZE',
            'MAX_FRAGMENT_UNIFORM_VECTORS',
            'MAX_RENDERBUFFER_SIZE',
            'MAX_TEXTURE_IMAGE_UNITS',
            'MAX_TEXTURE_SIZE',
            'MAX_VARYING_VECTORS',
            'MAX_VERTEX_ATTRIBS',
            'MAX_VERTEX_TEXTURE_IMAGE_UNITS',
            'MAX_VERTEX_UNIFORM_VECTORS',
            'MAX_VIEWPORT_DIMS',
            'RED_BITS',
            'RENDERER',
            'SHADING_LANGUAGE_VERSION',
            'STENCIL_BITS',
            'VERSION'
        ]

        const version2Constants = [
            'MAX_VARYING_COMPONENTS',
            'MAX_VERTEX_UNIFORM_COMPONENTS',
            'MAX_VERTEX_UNIFORM_BLOCKS',
            'MAX_VERTEX_OUTPUT_COMPONENTS',
            'MAX_PROGRAM_TEXEL_OFFSET',
            'MAX_3D_TEXTURE_SIZE',
            'MAX_ARRAY_TEXTURE_LAYERS',
            'MAX_COLOR_ATTACHMENTS',
            'MAX_COMBINED_FRAGMENT_UNIFORM_COMPONENTS',
            'MAX_COMBINED_UNIFORM_BLOCKS',
            'MAX_COMBINED_VERTEX_UNIFORM_COMPONENTS',
            'MAX_DRAW_BUFFERS',
            'MAX_ELEMENT_INDEX',
            'MAX_FRAGMENT_INPUT_COMPONENTS',
            'MAX_FRAGMENT_UNIFORM_COMPONENTS',
            'MAX_FRAGMENT_UNIFORM_BLOCKS',
            'MAX_SAMPLES',
            'MAX_SERVER_WAIT_TIMEOUT',
            'MAX_TEXTURE_LOD_BIAS',
            'MAX_TRANSFORM_FEEDBACK_INTERLEAVED_COMPONENTS',
            'MAX_TRANSFORM_FEEDBACK_SEPARATE_ATTRIBS',
            'MAX_TRANSFORM_FEEDBACK_SEPARATE_COMPONENTS',
            'MAX_UNIFORM_BLOCK_SIZE',
            'MAX_UNIFORM_BUFFER_BINDINGS',
            'MIN_PROGRAM_TEXEL_OFFSET',
            'UNIFORM_BUFFER_OFFSET_ALIGNMENT'
        ]

        const compileParameters = context => {
            try {
                const parameters = {
                    ANTIALIAS: context.getContextAttributes().antialias,
                    MAX_TEXTURE_MAX_ANISOTROPY_EXT: getMaxAnisotropy(context),
                    MAX_DRAW_BUFFERS_WEBGL: getMaxDrawBuffers(context),
                    VERTEX_SHADER: getShaderData('VERTEX_SHADER', getShaderPrecisionFormat(context, 'VERTEX_SHADER')),
                    FRAGMENT_SHADER: getShaderData('FRAGMENT_SHADER', getShaderPrecisionFormat(context, 'FRAGMENT_SHADER')),
                    UNMASKED_VENDOR_WEBGL: getUnmasked(context, 'UNMASKED_VENDOR_WEBGL'),
                    UNMASKED_RENDERER_WEBGL: getUnmasked(context, 'UNMASKED_RENDERER_WEBGL')
                }
                const pnames = type == 'webgl2' ? [...version1Constants, ...version2Constants] : version1Constants
                pnames.forEach(key => {
                    const value = context[key]
                    const result = context.getParameter(context[key])
                    const typedArray = (
                        result.constructor === Float32Array ||
                        result.constructor === Int32Array
                    )
                    parameters[key] = typedArray ? [...result] : result
                })
                return parameters
            } catch (error) {
                console.error(error)
                return undefined
            }
        }

        let getParameter = null
        try {
            getParameter = context.getParameter
        } catch (error) {}

        const parameters = !!getParameter ? compileParameters(context) : undefined
        const response = {
            parameters,
            supportedExtensions
        }
        if (type == 'webgl2') {
            response.newMethods = newMethods
        }
        return resolve(response)
    })
}