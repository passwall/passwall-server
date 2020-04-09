document.addEventListener('DOMContentLoaded', function() {
	var queryInfo = {
		active: true, 
		currentWindow: true
	};
	
	chrome.tabs.query(queryInfo, function(tabs) {
		var url = tabs[0].url;
		var domain = url.replace('file://','').replace('http://','').replace('https://','').split(/[/?#]/)[0];
		
		var xhr = new XMLHttpRequest();
		xhr.open("GET", "http://localhost:3625/logins/?Search="+domain, true);  //Mention your database query file here
		xhr.setRequestHeader("Authorization", "Basic " + btoa("gpass:password"));
		xhr.onreadystatechange = function() {
		
			if (xhr.readyState == 4) {
				var xhrjson = JSON.parse(xhr.responseText);
				xhrjson.forEach(arrayFunction);
				
				// chrome.tabs.executeScript(null,{code:"document.getElementById('username').value = '"+varxhrjson[0].Username+"'"});
				// chrome.tabs.executeScript(null,{code:"document.getElementById('password').value = '"+varxhrjson[0].Password+"'"});
			}
		}
		xhr.send();
	});
});

function arrayFunction(value, index, array) {
	var metin = value.Username + "\n" + value.Password+ "\n";
	alert(metin);
}