"use strict";

// CSRFトークンの取得
const getCSRFToken = () => {
    const metaToken = document.querySelector('meta[name="csrf-token"]')?.content;
    const headerToken = document.querySelector('header')?.dataset.csrfToken;
    const expectedToken = document.querySelector('header')?.getAttribute('data-csrf-token');
    return metaToken || headerToken || expectedToken;
};

document.addEventListener('DOMContentLoaded', () => {
    const video = document.getElementById('video');
    const overlay = document.getElementById('overlay');
    const startButton = document.getElementById('startButton');
    const captureButton = document.getElementById('captureButton');
    const stopButton = document.getElementById('stopButton');
    const result = document.getElementById('result');
    const primaryEmotion = document.getElementById('primaryEmotion');
    const confidence = document.getElementById('confidence');
    
    let stream = null;
    const ctx = overlay.getContext('2d');

    // ビデオのメタデータ読み込み完了時の処理
    video.addEventListener('loadedmetadata', () => {
        overlay.width = video.videoWidth;
        overlay.height = video.videoHeight;
    });

    // カメラの開始
    startButton.addEventListener('click', async () => {
        try {
            stream = await navigator.mediaDevices.getUserMedia({
                video: {
                    width: { ideal: 1280 },
                    height: { ideal: 720 }
                }
            });
            video.srcObject = stream;
            
            startButton.disabled = true;
            captureButton.disabled = false;
            stopButton.disabled = false;
            
            result.classList.add('hidden');
            ctx.clearRect(0, 0, overlay.width, overlay.height);
        } catch (err) {
            console.error('カメラの起動に失敗:', err);
            alert('カメラの起動に失敗しました。カメラへのアクセスを許可してください。');
        }
    });

    // 画像のキャプチャと分析
    captureButton.addEventListener('click', async () => {
        if (!stream) return;

        try {
            // キャンバスの準備
            const canvas = document.createElement('canvas');
            canvas.width = video.videoWidth || 640;
            canvas.height = video.videoHeight || 480;
            const context = canvas.getContext('2d');
            
            // 現在のフレームを描画
            context.drawImage(video, 0, 0, canvas.width, canvas.height);
            
            // Base64形式で画像を取得
            const imageData = canvas.toDataURL('image/jpeg', 0.9);
            
            // CSRFトークンを取得
            const csrfToken = getCSRFToken();
            if (!csrfToken) {
                throw new Error('CSRFトークンが見つかりません');
            }

            // リクエストボディの作成
            const requestBody = { image: imageData };
            console.log('送信するデータ:', {
                url: '/analyze',
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': csrfToken,
                    'X-Expected-CSRF-Token': csrfToken
                },
                bodyLength: imageData.length
            });

            // サーバーに画像を送信して分析
            const response = await fetch('/analyze', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': csrfToken,
                    'X-Expected-CSRF-Token': csrfToken
                },
                body: JSON.stringify(requestBody)
            });

            if (!response.ok) {
                const errorText = await response.text();
                console.error('サーバーエラー:', {
                    status: response.status,
                    statusText: response.statusText,
                    error: errorText
                });
                throw new Error(`分析に失敗しました: ${errorText}`);
            }

            const data = await response.json();
            
            // 結果の表示
            result.classList.remove('hidden');
            primaryEmotion.textContent = data.emotion;
            confidence.textContent = `${(data.confidence * 100).toFixed(1)}%`;

            // 検出された顔の領域を描画
            ctx.clearRect(0, 0, overlay.width, overlay.height);
            ctx.strokeStyle = '#00ff00';
            ctx.lineWidth = 2;
            
            for (const face of data.faces) {
                const x = face.x * overlay.width;
                const y = face.y * overlay.height;
                const width = face.width * overlay.width;
                const height = face.height * overlay.height;
                
                ctx.strokeRect(x, y, width, height);
                
                // 感情テキストを描画
                ctx.fillStyle = '#00ff00';
                ctx.font = '16px Arial';
                ctx.fillText(data.emotion, x, y - 5);
            }

        } catch (err) {
            console.error('分析エラー:', err);
            alert(`画像の分析中にエラーが発生しました: ${err.message}`);
        }
    });

    // カメラの停止
    stopButton.addEventListener('click', () => {
        if (stream) {
            stream.getTracks().forEach(track => track.stop());
            video.srcObject = null;
            stream = null;
            
            startButton.disabled = false;
            captureButton.disabled = true;
            stopButton.disabled = true;
            
            // オーバーレイをクリア
            ctx.clearRect(0, 0, overlay.width, overlay.height);
            result.classList.add('hidden');
        }
    });
});
