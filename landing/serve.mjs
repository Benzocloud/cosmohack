// Мини-статик-сервер с поддержкой Range (для честного видео-скраба в тестах).
import { createServer } from 'http';
import { readFile, stat } from 'fs/promises';
import { join, extname } from 'path';

const ROOT = new URL('.', import.meta.url).pathname;
const MIME = { '.html': 'text/html', '.css': 'text/css', '.js': 'text/javascript', '.mp4': 'video/mp4', '.jpg': 'image/jpeg', '.png': 'image/png', '.svg': 'image/svg+xml' };

createServer(async (req, res) => {
  const urlPath = decodeURIComponent(new URL(req.url, 'http://x').pathname);
  const file = join(ROOT, urlPath === '/' ? 'index.html' : urlPath);
  try {
    const s = await stat(file);
    if (!s.isFile()) throw new Error();
    const type = MIME[extname(file)] ?? 'application/octet-stream';
    const range = req.headers.range;
    if (range) {
      const m = range.match(/bytes=(\d*)-(\d*)/);
      let start = m[1] ? +m[1] : 0;
      let end = m[2] ? +m[2] : s.size - 1;
      end = Math.min(end, s.size - 1);
      res.writeHead(206, {
        'Content-Type': type,
        'Content-Range': `bytes ${start}-${end}/${s.size}`,
        'Accept-Ranges': 'bytes',
        'Content-Length': end - start + 1,
      });
      const buf = await readFile(file);
      res.end(buf.subarray(start, end + 1));
    } else {
      res.writeHead(200, { 'Content-Type': type, 'Accept-Ranges': 'bytes', 'Content-Length': s.size });
      res.end(await readFile(file));
    }
  } catch {
    res.writeHead(404);
    res.end('not found');
  }
}).listen(8642, () => console.log('serving on :8642'));
