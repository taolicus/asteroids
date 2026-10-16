const canvas = document.getElementById("arena");
const ctx = canvas.getContext("2d");

function resize() {
  canvas.width = canvas.clientWidth * devicePixelRatio;
  canvas.height = canvas.clientHeight * devicePixelRatio;
}

addEventListener("resize", resize);
resize();
