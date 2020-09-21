import os
badchar = [' ','^','$']
fs = os.listdir()
wordset = set()
enc = input("Write encoding (default: 'utf-8'. use '1252' for windows files) ") 
enc = "utf-8" if enc == "" else enc
for f in fs:
	last=f[len(f)-4:len(f)]
	if last == ".dic":
		YorN = input(f"Do you wish to merge {f}? (y/N)")
		if YorN.lower() == "y":
			with open(f,encoding=enc) as fp:# exclude first line (contains number of entries)
				words = fp.readlines()
				for i in range(len(words)):
					for char in badchar:
						words[i] = words[i].replace(char,'')
				wordset |= set(words)
wordlist=list(wordset)
wordlist.sort()
wordlist = [str(len(wordset))+"\n"] + wordlist#wordlist.insert(0, str(len(wordset)))
open(input("output dictionary name: "),"w+",encoding='utf-8').writelines(wordlist)