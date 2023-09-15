let express = require('express');
let path = require('path');
let fs = require('fs');
let app = express();


app.get('/', function (req, res) {
    res.sendFile(path.join(__dirname, "index.html"));
});

app.get('/profile-picture-andrea', function (req, res) {
  let img = fs.readFileSync(path.join(__dirname, "images/profile-andrea.jpg"));
  res.writeHead(200, {'Content-Type': 'image/jpg' });
  res.end(img, 'binary');
});

app.get('/profile-picture-ari', function (req, res) {
  let img = fs.readFileSync(path.join(__dirname, "images/profile-ari.jpeg"));
  res.writeHead(200, {'Content-Type': 'image/jpg' });
  res.end(img, 'binary');
});

app.listen(3000, function () {
  console.log("app listening on port 3000!");
});