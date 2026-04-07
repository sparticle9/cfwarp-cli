import http from 'k6/http';
import { check, sleep } from 'k6';

const vus = Number(__ENV.VUS || 16);
const duration = __ENV.DURATION || '30s';
const method = (__ENV.METHOD || 'GET').toUpperCase();
const targetUrl = __ENV.TARGET_URL;
const bodyBytes = Number(__ENV.BODY_BYTES || 0);
const thinkMs = Number(__ENV.THINK_MS || 0);
const noConnectionReuse = (__ENV.NO_CONNECTION_REUSE || '0') === '1';
const timeout = __ENV.TIMEOUT || '60s';

function makeBody(size) {
  if (!size || size <= 0) return '';
  return 'x'.repeat(size);
}

const body = makeBody(bodyBytes);

export const options = {
  vus,
  duration,
  noConnectionReuse,
  userAgent: 'cfwarp-bench-k6/1.0',
  insecureSkipTLSVerify: true,
  discardResponseBodies: false,
  summaryTrendStats: ['avg', 'med', 'p(95)', 'p(99)', 'max', 'min'],
};

export default function () {
  const params = {
    timeout,
    headers: {
      'Content-Type': 'application/octet-stream',
      'X-Bench-Profile': __ENV.PROFILE || 'unknown',
    },
  };

  let res;
  if (method === 'GET') {
    res = http.get(targetUrl, params);
  } else {
    res = http.request(method, targetUrl, body, params);
  }

  check(res, {
    'status is 200': (r) => r.status === 200,
  });

  if (thinkMs > 0) {
    sleep(thinkMs / 1000);
  }
}
