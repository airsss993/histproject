package sites

import (
	"bytes"
	"path"
	"strings"
)

// scrollScript сообщает странице /event/:id, как далеко долистан сайт. Сайт открыт в iframe с sandbox
// без allow-same-origin, снаружи его прокрутку не прочитать, поэтому скрипт живёт в самом HTML.
// Мерить начинаем с load: до загрузки картинок страница короче и выглядела бы дочитанной.
const scrollScript = `<script data-hp-scroll>(function(){var m=0;function s(){var d=document.documentElement,` +
	`h=Math.max(d.scrollHeight,document.body?document.body.scrollHeight:0),` +
	`p=h<=innerHeight+1?100:Math.min(100,Math.round((scrollY+innerHeight)/h*100));` +
	`if(p>m){m=p;parent.postMessage({hpScroll:m},'*')}}` +
	`addEventListener('scroll',s,{passive:true});addEventListener('load',s)})()</script>`

var scrollMarker = []byte("data-hp-scroll")

// InjectScrollScript дописывает скрипт прокрутки в конец HTML. Повторно не дописывает.
func InjectScrollScript(html []byte) ([]byte, bool) {
	if bytes.Contains(html, scrollMarker) {
		return html, false
	}
	return append(html, scrollScript...), true
}

func IsHTML(filePath string) bool {
	extension := strings.ToLower(path.Ext(filePath))
	return extension == ".html" || extension == ".htm"
}
