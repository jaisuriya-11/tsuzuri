'use strict';
// Finding, running and installing the tsuzuri binary.

const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFile, spawn } = require('child_process');

const INSTALL_SH = 'https://raw.githubusercontent.com/jaisuriya-11/tsuzuri/main/install.sh';
const INSTALL_PS1 = 'https://raw.githubusercontent.com/jaisuriya-11/tsuzuri/main/install.ps1';

const isWindows = process.platform === 'win32';

function isFile(p) {
  try {
    return fs.statSync(p).isFile();
  } catch {
    return false;
  }
}

/** Expands a leading ~ and ${workspaceFolder} in a user-supplied path. */
function expandPath(p, workspaceFolder) {
  if (!p) return '';
  let out = p.trim();
  if (out === '~' || out.startsWith('~/') || out.startsWith('~\\')) {
    out = path.join(os.homedir(), out.slice(1));
  }
  if (workspaceFolder) out = out.split('${workspaceFolder}').join(workspaceFolder);
  return out;
}

/**
 * Every place the binary may live. VS Code's own PATH is fixed when it
 * starts, so a fresh install into ~/.local/bin or %LOCALAPPDATA% would be
 * missed without the explicit folders below.
 */
function candidates() {
  const exe = isWindows ? 'tsuzuri.exe' : 'tsuzuri';
  const home = os.homedir();
  const dirs = (process.env.PATH || '').split(path.delimiter).filter(Boolean);
  dirs.push(
    path.join(home, '.local', 'bin'),
    '/usr/local/bin',
    '/opt/homebrew/bin',
    path.join(process.env.GOBIN || path.join(process.env.GOPATH || path.join(home, 'go'), 'bin'))
  );
  if (isWindows && process.env.LOCALAPPDATA) {
    dirs.push(path.join(process.env.LOCALAPPDATA, 'Programs', 'tsuzuri'));
  }
  return [...new Set(dirs)].map((d) => path.join(d, exe));
}

/** Returns the binary path, or undefined when it can't be found. */
function find(configuredPath) {
  if (configuredPath) return isFile(configuredPath) ? configuredPath : undefined;
  return candidates().find(isFile);
}

/** Runs the binary and resolves with stdout. */
function run(bin, args, cwd) {
  return new Promise((resolve, reject) => {
    execFile(bin, args, { cwd, timeout: 60_000, windowsHide: true }, (err, stdout, stderr) => {
      if (err) {
        const detail = (stderr || '').trim() || err.message;
        reject(new Error(detail));
      } else {
        resolve(stdout);
      }
    });
  });
}

/**
 * Sends one "tsuzuri editor <command>" request: payload goes in as JSON on
 * stdin and the parsed JSON response comes back.
 */
function request(bin, command, payload, timeout = 15_000) {
  return new Promise((resolve, reject) => {
    const child = spawn(bin, ['editor', command], { windowsHide: true });
    const out = [];
    const err = [];
    const timer = setTimeout(() => {
      child.kill();
      reject(new Error(`tsuzuri editor ${command} timed out`));
    }, timeout);
    child.stdout.on('data', (d) => out.push(d));
    child.stderr.on('data', (d) => err.push(d));
    child.on('error', (e) => {
      clearTimeout(timer);
      reject(e);
    });
    child.on('close', (code) => {
      clearTimeout(timer);
      if (code !== 0) {
        const detail = Buffer.concat(err).toString().trim();
        reject(new Error(detail || `tsuzuri editor ${command} exited with code ${code}`));
        return;
      }
      try {
        resolve(JSON.parse(Buffer.concat(out).toString()));
      } catch {
        reject(new Error(`tsuzuri editor ${command} returned an unreadable response`));
      }
    });
    // A binary that exits early closes stdin; the close handler reports why.
    child.stdin.on('error', () => {});
    child.stdin.end(payload === undefined ? '' : JSON.stringify(payload));
  });
}

/** The shell and command that run the official install script. */
function installCommand() {
  if (isWindows) {
    return { shellPath: 'powershell.exe', text: `irm ${INSTALL_PS1} | iex` };
  }
  return { shellPath: undefined, text: `curl -fsSL ${INSTALL_SH} | sh` };
}

module.exports = { expandPath, find, run, request, installCommand };
