/* Threads: light, not paint. Every color here is linear, and threads add up in a float buffer. */
const threadVert = `
attribute float aU, aR, aJ;
uniform float uT, uN, uE, uLevel; uniform vec2 uFit;
varying float vA; varying vec3 vC;
vec3 pal(vec3 a,vec3 b,vec3 c,float t){ return t<.5?mix(a,b,t*2.):mix(b,c,(t-.5)*2.); }
void main(){
  float u=aU*6.2831853;
  float wob=.075+.05*uE;
  float r=.98+wob*sin(3.*u+aR*1.7+aJ*3.+uT*.7)+.022*sin(7.*u-uT+aJ*6.);
  float ang=.9+aJ*.95+aR*.5+.34*sin(uT*.45+aR*1.3);
  float x=r*cos(u), y=r*sin(u)*cos(ang), z=r*sin(u)*sin(ang);
  float tilt=.35+aR*.5236+uT*(.05+.018*aR);
  vec3 p=vec3(x*cos(tilt)-y*sin(tilt),x*sin(tilt)+y*cos(tilt),z);
  float a=uT*.11; p=vec3(p.x*cos(a)+p.z*sin(a),p.y,-p.x*sin(a)+p.z*cos(a));
  // Hue wanders along each thread and across each ribbon, so no two lines match.
  float h=.5+.5*sin(6.2831853*(aR*.37+aJ*.8+aU+uT*.02));
  vec3 cool=pal(vec3(.02,.10,.88),vec3(.03,.30,.98),vec3(.12,.05,.90),h);
  vec3 grown=pal(vec3(.00,.52,.46),vec3(.01,.60,.14),vec3(.28,.74,.05),h);
  // A wide, soft front: the two lights overlap across most of the sphere on the way.
  float lvl=uLevel+.05*sin(p.x*4.+uT*.8);
  float front=smoothstep(.62,-.62,p.y-lvl);
  vC=mix(cool,grown,front);
  float depth=p.z*.5+.5;
  vA=clamp(uN-aR,0.,1.)*(.24+.24*depth)*(1.+.2*uE);
  gl_Position=vec4(p.xy*.84*uFit,p.z*.1,1.);
}`;
const threadFrag = `
precision highp float;
varying float vA; varying vec3 vC;
void main(){ gl_FragColor=vec4(vC*vA,vA); }`;
const lightFrag = `
precision highp float;
uniform sampler2D tLight; varying vec2 vUv;
vec3 toDisplay(vec3 c){ return mix(c*12.92,1.055*pow(max(c,vec3(.0001)),vec3(1./2.4))-.055,step(.0031308,c)); }
void main(){
  vec4 a=texture2D(tLight,vUv);
  float d=a.a;
  if(d<.0005){ gl_FragColor=vec4(0.); return; }
  // Crossing threads mix as lights do: their colors average in linear, and they read denser.
  vec3 lin=a.rgb/d;
  lin*=1.+.3*(1.-exp(-d*1.2));
  float cover=1.-exp(-d*1.15);
  gl_FragColor=vec4(min(toDisplay(lin),vec3(1.))*cover,cover);
}`;

/* Render. It starts once the first state is known, so a screen opened at rest draws at its own progress. */
function startThreads() {
const stage = $("stage"), canvas = $("gl");
let renderer = null;
try { renderer = new THREE.WebGLRenderer({ canvas, alpha: true, antialias: true, powerPreference: "low-power" }); } catch (e) {}

if (atRest) stage.style.transition = "none";
if (!atRest) canvas.animate([{ opacity: 0 }, { opacity: 1 }], { duration: 1600, easing: EASE });

if (renderer) {
  renderer.setClearColor(0x000000, 0);
  const cam = new THREE.Camera();
  const threadU = { uT: { value: 0 }, uN: { value: 3 }, uE: { value: 0 }, uLevel: { value: -2 }, uFit: { value: new THREE.Vector2(1, 1) } };
  const threadScene = new THREE.Scene();
  {
    const RIB = 6, STR = 22, SEG = 180, pos = [], au = [], ar = [], aj = [], idx = [];
    for (let r = 0; r < RIB; r++) for (let j = 0; j < STR; j++) {
      const base = au.length;
      for (let i = 0; i < SEG; i++) { pos.push(0, 0, 0); au.push(i / SEG); ar.push(r); aj.push(j / STR); }
      for (let i = 0; i < SEG; i++) idx.push(base + i, base + (i + 1) % SEG);
    }
    const geo = new THREE.BufferGeometry();
    geo.setAttribute("position", new THREE.Float32BufferAttribute(pos, 3));
    geo.setAttribute("aU", new THREE.Float32BufferAttribute(au, 1));
    geo.setAttribute("aR", new THREE.Float32BufferAttribute(ar, 1));
    geo.setAttribute("aJ", new THREE.Float32BufferAttribute(aj, 1));
    geo.setIndex(idx);
    const mesh = new THREE.LineSegments(geo, new THREE.ShaderMaterial({
      uniforms: threadU, vertexShader: threadVert, fragmentShader: threadFrag, transparent: true, depthTest: false, depthWrite: false,
      blending: THREE.CustomBlending, blendSrc: THREE.OneFactor, blendDst: THREE.OneFactor,
    }));
    mesh.frustumCulled = false;
    threadScene.add(mesh);
  }

  // Half-float keeps faint threads from banding; where it's missing, eight bits still works.
  const gl = renderer.getContext();
  const floatOK = renderer.capabilities.isWebGL2 && !!gl.getExtension("EXT_color_buffer_float");
  const RT = renderer.capabilities.isWebGL2 ? THREE.WebGLMultisampleRenderTarget : THREE.WebGLRenderTarget;
  const light = new RT(4, 4, { type: floatOK ? THREE.HalfFloatType : THREE.UnsignedByteType, format: THREE.RGBAFormat, depthBuffer: false, stencilBuffer: false });
  const lightScene = new THREE.Scene();
  lightScene.add(new THREE.Mesh(new THREE.PlaneBufferGeometry(2, 2), new THREE.ShaderMaterial({
    uniforms: { tLight: { value: light.texture } },
    vertexShader: "varying vec2 vUv; void main(){ vUv=uv; gl_Position=vec4(position.xy,0.,1.); }",
    fragmentShader: lightFrag, blending: THREE.NoBlending, depthTest: false, depthWrite: false,
  })));

  function size() {
    const w = stage.clientWidth, h = stage.clientHeight;
    const dpr = Math.min(devicePixelRatio || 1, 1.75);
    renderer.setPixelRatio(dpr);
    renderer.setSize(w, h, false);
    threadU.uFit.value.set(Math.min(w, h) / w, Math.min(w, h) / h);
    light.setSize(Math.round(w * dpr), Math.round(h * dpr));
  }
  new ResizeObserver(size).observe(stage);
  size();

  let last = performance.now(), T = 7, speed = working ? WORK_SPEED : WAIT_SPEED, energy = working ? 1 : 0, shown = progress, ribbons = settled ? 3 : 3 + 3 * progress;
  function frame(now) {
    const dt = Math.min((now - last) / 1000, .05); last = now;
    const blend = 1 - Math.exp(-dt * (working ? 3.2 : settled ? .55 : 1.1));
    const wantSpeed = working ? WORK_SPEED : (settled ? SETTLED_SPEED : WAIT_SPEED);
    speed += (wantSpeed * (calm ? .1 : 1) - speed) * blend;
    energy += ((working ? 1 : 0) - energy) * blend;
    shown += (progress - shown) * (1 - Math.exp(-dt * 1.2));
    ribbons += ((settled ? 3 : 3 + 3 * progress) - ribbons) * (1 - Math.exp(-dt * .7));
    T += dt * speed;

    threadU.uT.value = T; threadU.uN.value = ribbons; threadU.uE.value = energy;
    threadU.uLevel.value = -1.75 + 3.5 * shown;
    renderer.setRenderTarget(light);
    renderer.clear();
    renderer.render(threadScene, cam);
    renderer.setRenderTarget(null);
    renderer.render(lightScene, cam);
    $("glow").style.opacity = (.9 * shown).toFixed(3);
    const aura = [88 - 60 * shown, 136 + 54 * shown, 232 - 118 * shown].map(Math.round).join(" ");
    document.documentElement.style.setProperty("--aura", aura);
    const pct = $("pct");
    pct.textContent = Math.round(shown * 100) + "% complete";
    pct.style.opacity = shown > .005 && !settled ? 1 : 0;
    requestAnimationFrame(frame);
  }
  frame(performance.now());
}
}
