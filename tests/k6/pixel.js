import http from 'k6/http';
import { sleep } from 'k6';

export default function () {
    // http.get('http://localhost:8080/ping');
    http.post('http://localhost:8080/pixel', JSON.stringify({
        x: Math.floor(Math.random() * 1000),
        y: Math.floor(Math.random() * 1000),
        color: Math.floor(Math.random() * 32)
    }));
}
