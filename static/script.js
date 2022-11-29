function reqListener() {
        var data = JSON.parse(this.responseText);
        flightMarkerLayer.clearLayers();
        for (const f of data.flights) {
                flightMarkerLayer.addLayer(L.circleMarker(L.latLng(f.lat, f.lon), { radius: 5, color: "#00aaff" }))
        }
}

let radius = 300;
let lat = 51.505;
let lon = 19.5;

document.getElementById('longitude').value = lon
document.getElementById('latitude').value = lat

const map = L.map('map').setView([lat, lon], 13);

const circle = L.circle([lat, lon], { radius: radius * 1000, fill: false, color: "#222222" }).addTo(map);

const flightMarkerLayer = L.layerGroup().addTo(map);

document.getElementById('range').oninput = function () {
        radius = document.getElementById('range').value
        document.getElementById('range-label').innerHTML = "Radius: " + radius
        circle.setRadius(radius * 1000)
}


const tiles = L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
        maxZoom: 19,
        attribution: '&copy; <a href="http://www.openstreetmap.org/copyright">OpenStreetMap</a>'
}).addTo(map);

const req = new XMLHttpRequest();
req.addEventListener("load", reqListener);

setInterval(function () {
        req.open("GET", "/flights?lat=" + lat + "&lon=" + lon + "&radius=" + radius);
        req.send();
}, 5000);

map.on('click', function (e) {
        var latlng = e.latlng;
        console.log(latlng);
        lat = latlng.lat;
        lon = latlng.lng;
        circle.setLatLng(latlng);
        document.getElementById('longitude').value = lon
        document.getElementById('latitude').value = lat

})