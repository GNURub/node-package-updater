#!/bin/bash
echo "Preparing environment..."
rm -rf node_modules
rm -rf package-lock.json
rm -rf package.json

npm i npm-check-updates -g

cat << EOF > ./package.json
{
  "name": "test",
  "version": "0.0.0",
  "private": true,
  "dependencies": {
    "@angular/animations": "^15.0.0",
    "@angular/cdk": "^15.0.0",
    "@angular/common": "^15.0.0",
    "@angular/compiler": "^15.0.0",
    "@angular/core": "^15.0.0",
    "@angular/elements": "^15.0.0",
    "@angular/forms": "^15.0.0",
    "@angular/localize": "^15.0.0",
    "@angular/material": "^15.0.0",
    "@angular/platform-browser": "~15.0.0",
    "@angular/platform-browser-dynamic": "^15.0.0",
    "@angular/platform-server": "^15.0.0",
    "@angular/router": "~15.0.0",
    "@angular/service-worker": ">=15.0.0",
    "nomnom": "^0.0.1",
    "@cesdk/cesdk-js": "^1.43.0",
    "@ctrl/ngx-emoji-mart": "9.2.0",
    "@ffmpeg/ffmpeg": "^0.12.15",
    "@ffmpeg/util": "^0.12.2",
    "@fingerprintjs/fingerprintjs": "~4.5.1",
    "@fortawesome/angular-fontawesome": "^1.0.0",
    "@fortawesome/fontawesome-free": "^6.7.2",
    "@fortawesome/fontawesome-svg-core": "^6.7.2",
    "@fortawesome/free-brands-svg-icons": "^6.7.2",
    "@fortawesome/free-solid-svg-icons": "^6.7.2",
    "@jsverse/transloco": "7.5.1",
    "@mapbox/mapbox-gl-geocoder": "5.0.3",
    "@ng-bootstrap/ng-bootstrap": "^18.0.0",
    "@ng-select/ng-select": "14.2.0",
    "@ng-web-apis/common": "^4.11.1",
    "@ng-web-apis/universal": "^4.11.1",
    "@ngneat/until-destroy": "10.0.0",
    "@ngrx/operators": "^19.0.0",
    "@ngrx/signals": "^19.0.0",
    "@nguniversal/express-engine": "16.2.0",
    "@silvermine/videojs-airplay": "1.3.0",
    "@silvermine/videojs-quality-selector": "1.3.1",
    "@stripe/stripe-js": "^5.5.0",
    "@sweetalert2/ngx-sweetalert2": "12.4.0",
    "@swimlane/ngx-datatable": "^20.1.0",
    "@types/swiper": "^6.0.0",
    "@types/ua-parser-js": "^0.7.39",
    "@zxing/browser": "^0.1.5",
    "@zxing/library": "^0.21.3",
    "@zxing/ngx-scanner": "19.0.0",
    "algoliasearch": "5.20.0",
    "algoliasearch-helper": "^3.23.1",
    "angularx-qrcode": "^19.0.0",
    "animate.css": "0.0.0",
    "bootstrap": "^5.3.3",
    "buffer": "^6.0.3",
    "centrifuge": "^5.3.0",
    "chart.js": "4.4.7",
    "chartist": "1.3.0",
    "chartjs-plugin-datalabels": "2.2.0",
    "compression": "1.7.5",
    "compressorjs": "^1.2.1",
    "date-fns": "4.1.0",
    "date-fns-timezone": "0.1.4",
    "dayjs": "1.11.13",
    "device-uuid": "1.0.4",
    "driver.js": "^1.3.1",
    "eva-icons": "1.1.3",
    "express": "5.0.1",
    "hark": "^1.2.3",
    "hls.js": "^1.5.20",
    "instantsearch.js": "4.77.1",
    "isbot": "^5.1.21",
    "lazysizes": "5.3.2",
    "linkify-html": "4.2.0",
    "linkify-plugin-mention": "4.2.0",
    "linkifyjs": "4.2.0",
    "lodash": "4.17.21",
    "lottie-web": "^5.12.2",
    "mapbox-gl": "^3.9.3",
    "mapbox.js": "3.3.1",
    "marked": "^15.0.6",
    "moment": "^2.30.1",
    "ng-otp-input": "2.0.6",
    "ng2-charts": "8.0.0",
    "ts-cacheable": "1.0.10",
    "ngx-captcha": "13.0.0",
    "ngx-clipboard": "16.0.0",
    "ngx-cookie-service": "19.0.0",
    "ngx-infinite-scroll": "19.0.0",
    "ngx-lottie": "^13.0.0",
    "ngx-owl-carousel-o": "^19.0.0",
    "ngx-page-scroll-core": "13.0.0",
    "ngx-progressbar": "14.0.0",
    "ngx-sharebuttons": "16.0.0",
    "ngx-toastr": "19.0.0",
    "ngxtension": "^4.2.1",
    "object-to-formdata": "^4.5.1",
    "plyr": "3.7.8",
    "pretty-checkbox": "3.0.3",
    "random-gradient": "0.0.2",
    "randomcolor": "0.6.2",
    "rxjs": "~7.8.1",
    "sanitize-html": "^2.14.0",
    "simplebar-angular": "3.3.0",
    "socket.io-client": "^4.8.1",
    "sweetalert2": "11.15.10",
    "swiper": "11.2.1",
    "timezone-support": "3.1.0",
    "tippy.js": "6.3.7",
    "twemoji": "^14.0.2",
    "ua-parser-js": "^2.0.0",
    "wasm-check": "^2.1.2",
    "wavesurfer.js": "7.8.16",
    "xlsx": "^0.18.5",
    "zone.js": "~0.15.0"
  },
  "devDependencies": {
    "@angular-devkit/build-angular": "^19.1.4",
    "@angular-eslint/builder": "19.0.2",
    "@angular-eslint/eslint-plugin": "19.0.2",
    "@angular-eslint/eslint-plugin-template": "19.0.2",
    "@angular-eslint/schematics": "19.0.2",
    "@angular-eslint/template-parser": "19.0.2",
    "@angular/cli": "^19.1.4",
    "@angular/compiler-cli": "^19.1.3",
    "@types/emscripten": "^1.39.13",
    "@types/express": "5.0.0",
    "@types/jasmine": "~5.1.5",
    "@types/jasminewd2": "2.0.13",
    "@types/mapbox-gl": "^3.4.1",
    "@types/node": "22.10.9",
    "@typescript-eslint/eslint-plugin": "8.21.0",
    "@typescript-eslint/parser": "8.21.0",
    "eslint": "^9.18.0",
    "jasmine-core": "~5.5.0",
    "karma": "~6.4.4",
    "karma-chrome-launcher": "~3.2.0",
    "karma-coverage": "~2.2.1",
    "karma-jasmine": "~5.1.0",
    "karma-jasmine-html-reporter": "~2.1.0",
    "prettier": "^3.4.2",
    "stripe": "^17.5.0",
    "tailwindcss": "^4.0.0",
    "typescript": "~5.7.3",
    "tslib": "^2.8.1"
  }
}
EOF