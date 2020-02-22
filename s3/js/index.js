// ランキングHTML生成
function makeListHTML(data){
	var html = "";
	if(0 == data.length){
		html += '<p class="text-center err-message py-5">該当記事がありませんでした。</p>';
	} else {
		var title = data[0].Tag == "overall" ? "総合" : data[0].Tag;
		html += '<div class="card-header">'+ title + 'ランキング</div>';
		html += '<div class="list-group list-group-flush">';
		for (var i = 0; i < data.length; i++) {
			html += '<a class="list-group-item text-decoration-none" target="_blank" href="' + data[i].URL + '">';
			html += '<div class="pb-3">' + data[i].Title + '</div>';
			html += '<div>';
			html += '<span class="likes-count mr-2"><i class="fas fa-thumbs-up"></i>' + data[i].LikesCount + '</span>'
			for (var j=0; j < data[i].Tags.length; j++) {
				html += '<span class="btn btn-sm btn-tags mx-1">' + data[i].Tags[j].Name + '</span>';
			}
			html += '</div>';
			html += '</a>';
		}
		html += '</div>';
	}
	return html;
}

// APIリクエスト送信
function requestAPI(tag){
	// Ajax通信を開始する
	$.ajax({
		url: 'https://pr31nk830m.execute-api.ap-northeast-1.amazonaws.com/prod/' + tag,
		type: 'get',
		dataType: 'json',
	})
	// レスポンスデータでHTMLを生成
	.done(function (response) {
		$("#ranking").html(makeListHTML(response));
	});
}

$(function () {
	// APIを叩く（総合ランキング取得）
	requestAPI("overall");
	// タグリンク押下時
	$('span.tag-link').click(function () {
		// ハンバーガーメニューを閉じる
		var $checkbox = $('input[type="checkbox"]');
		$checkbox.removeAttr('checked').prop('checked', false).change();
		// APIを叩く（タグランキング取得）
		requestAPI($(this).text());
    });
});